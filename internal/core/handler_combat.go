package core

import (
	"encoding/binary"
	"encoding/json"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/Jx2f/ViaGenshin/internal/mapper"
	"github.com/Jx2f/ViaGenshin/pkg/alg"
	"github.com/Jx2f/ViaGenshin/pkg/logger"
)

type CombatInvokeEntry struct {
	CombatData   []byte `json:"combatData"`
	ArgumentType uint32 `json:"argumentType"`
	ForwardType  uint32 `json:"forwardType"`
}

type CombatInvocationsNotify struct {
	InvokeList []*CombatInvokeEntry `json:"invokeList"`
}

func (s *Session) OnCombatInvocationsNotify(from, to mapper.Protocol, data []byte) ([]byte, error) {
	notify := new(CombatInvocationsNotify)
	err := json.Unmarshal(data, notify)
	if err != nil {
		return data, err
	}
	notify.InvokeList = s.OnCombatInvocations(from, to, notify.InvokeList)
	return json.Marshal(notify)
}

func (s *Session) OnCombatInvocations(from, to mapper.Protocol, in []*CombatInvokeEntry) []*CombatInvokeEntry {
	var out []*CombatInvokeEntry
	var err error
	for _, invoke := range in {
		if len(invoke.CombatData) == 0 {
			out = append(out, invoke)
			continue
		}
		name := mapper.CombatTypeArguments[invoke.ArgumentType]
		if name == "" {
			logger.Debug("Unknown combat invoke packet %d", invoke.ArgumentType)
			continue
		}
		invoke.CombatData, err = s.ConvertPacketByName(from, to, name, invoke.CombatData)
		if err != nil {
			logger.Debug("Failed to convert combat invoke packet %s, err: %v", name, err)
			continue
		}
		out = append(out, invoke)
	}
	return out
}

/****************************** 地形采集 ******************************/

type Vector3 struct {
	X float32 `json:"x"`
	Y float32 `json:"y"`
	Z float32 `json:"z"`
}

type MotionInfo struct {
	Pos   *Vector3 `json:"pos"`
	Rot   *Vector3 `json:"rot"`
	Speed *Vector3 `json:"speed"`
	State uint32   `json:"state"`
}

type EntityMoveInfo struct {
	EntityId   uint32      `json:"entityId"`
	MotionInfo *MotionInfo `json:"motionInfo"`
}

func (s *Session) HandleEntityMoveInfo(data []byte) {
	if s.playerSceneId != 3 {
		// 不在大世界场景
		return
	}
	entityMoveInfo := new(EntityMoveInfo)
	err := json.Unmarshal(data, entityMoveInfo)
	if err != nil {
		// 解析失败
		return
	}
	if (entityMoveInfo.EntityId >> 24) != 1 {
		// 非玩家角色实体在移动
		return
	}
	motionInfo := entityMoveInfo.MotionInfo
	if motionInfo == nil {
		return
	}
	if motionInfo.State != 4 && motionInfo.State != 5 && motionInfo.State != 6 && motionInfo.State != 7 {
		// 非与地形接触运动状态
		return
	}
	if motionInfo.Pos == nil || motionInfo.Rot == nil || motionInfo.Speed == nil {
		return
	}
	if motionInfo.Speed.X == 0.0 && motionInfo.Speed.Y == 0.0 && motionInfo.Speed.Z == 0.0 {
		return
	}
	pos := motionInfo.Pos
	// ----------只读访问并发安全操作----------
	gid := s.AoiManager.GetGidByPos(pos.X, 0.0, pos.Z)
	terrain := s.TerrainMap[gid]
	// ----------只读访问并发安全操作----------
	terrain.Lock.Lock()
	terrain.MeshPosMap[MeshPos{X: int16(pos.X), Y: int16(pos.Y), Z: int16(pos.Z)}] = struct{}{}
	terrain.Lock.Unlock()
}

type MeshPos struct {
	X int16
	Y int16
	Z int16
}

type Terrain struct {
	MeshPosMap map[MeshPos]struct{}
	Lock       sync.Mutex
}

func (s *Service) InitTerrain() {
	s.AoiManager = alg.NewAoiManager()
	s.AoiManager.SetAoiRange(-7168, 3072, -1, 1, -5120, 7168)
	s.AoiManager.Init3DRectAoiManager(10, 1, 12)
	s.TerrainMap = make(map[uint32]*Terrain)
	s.LoadTerrain()
	s.SaveTerrain()
	go func() {
		ticker := time.NewTicker(time.Minute * 10)
		for {
			<-ticker.C
			logger.Info("save terrain data start")
			s.SaveTerrain()
			logger.Info("save terrain data end")
		}
	}()
}

func (s *Service) LoadTerrain() {
	for gid := 0; gid < 10*1*12; gid++ {
		terrain := &Terrain{
			MeshPosMap: make(map[MeshPos]struct{}),
		}
		s.TerrainMap[uint32(gid)] = terrain
		data, err := os.ReadFile("./terrain/grid_" + strconv.Itoa(gid) + ".terr")
		if err != nil {
			logger.Error("load terrain data error: %v", err)
			continue
		}
		for i := 0; i < len(data); i += 8 {
			if data[i*8+0] != 0xAA || data[i*8+7] != 0xFF {
				logger.Error("decode terrain data format error")
				break
			}
			terrain.MeshPosMap[MeshPos{
				X: int16(binary.BigEndian.Uint16(data[i*8+1 : i*8+3])),
				Y: int16(binary.BigEndian.Uint16(data[i*8+3 : i*8+5])),
				Z: int16(binary.BigEndian.Uint16(data[i*8+5 : i*8+7])),
			}] = struct{}{}
		}
	}
}

func (s *Service) SaveTerrain() {
	_ = os.Mkdir("terrain", 0644)
	for gid := 0; gid < 10*1*12; gid++ {
		terrain := s.TerrainMap[uint32(gid)]
		terrain.Lock.Lock()
		data := make([]byte, len(terrain.MeshPosMap)*8)
		offset := 0
		for meshPos := range terrain.MeshPosMap {
			data[offset] = 0xAA
			offset++
			binary.BigEndian.PutUint16(data[offset:offset+2], uint16(meshPos.X))
			offset += 2
			binary.BigEndian.PutUint16(data[offset:offset+2], uint16(meshPos.Y))
			offset += 2
			binary.BigEndian.PutUint16(data[offset:offset+2], uint16(meshPos.Z))
			offset += 2
			data[offset] = 0xFF
			offset++
		}
		terrain.Lock.Unlock()
		err := os.WriteFile("./terrain/grid_"+strconv.Itoa(gid)+".terr", data, 0644)
		if err != nil {
			logger.Error("save terrain data error: %v", err)
			continue
		}
	}
}

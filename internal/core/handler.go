package core

import (
	"encoding/json"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/Jx2f/ViaGenshin/internal/config"
	"github.com/Jx2f/ViaGenshin/internal/mapper"
	"github.com/Jx2f/ViaGenshin/pkg/logger"
)

type PlayerLuaShellNotify struct {
	Id        uint32 `json:"id"`
	ShellType uint32 `json:"shell_type"`
	UseType   uint32 `json:"use_type"`
	LuaShell  []byte `json:"lua_shell"`
}

var LuaShellCode [][]byte = nil

const LuaPathPrefix = "./data/lua/"

func LoadLuaShellCode() {
	luaShellCode := make([][]byte, 0)
	for _, fileName := range config.GetConfig().LuaShellFile {
		split := strings.Split(fileName, ".")
		if len(split) != 2 || split[1] != "lua" {
			logger.Error("not lua file, fileName: %v", fileName)
			continue
		}
		name := split[0]
		exe := "luac_hk4e"
		if runtime.GOOS == "windows" {
			exe += ".exe"
		}
		command := exec.Command(exe, "-o", LuaPathPrefix+name+".luac", LuaPathPrefix+name+".lua")
		output, err := command.CombinedOutput()
		if err != nil {
			logger.Error("build luac file error: %v, fileName: %v, try load old file", err, fileName)
		} else {
			logger.Info("build luac file ok, output: %v, fileName: %v", string(output), fileName)
		}
		data, err := os.ReadFile(LuaPathPrefix + name + ".luac")
		if err != nil {
			logger.Error("read luac file error: %v, fileName: %v", err, fileName)
			continue
		}
		luaShellCode = append(luaShellCode, data)
		logger.Info("load luac file: %v", LuaPathPrefix+name+".luac")
	}
	LuaShellCode = luaShellCode
}

func (s *Session) SendLuaShellCode(shellCode []byte) {
	ntf := &PlayerLuaShellNotify{
		Id:        1,
		ShellType: 1,
		UseType:   1,
		LuaShell:  shellCode,
	}
	data, err := json.Marshal(ntf)
	if err != nil {
		logger.Error("marshal json error: %v", err)
		return
	}
	err = s.SendPacketJSON(s.endpoint, s.protocol, "PlayerLuaShellNotify", nil, data)
	if err != nil {
		logger.Warn("exit tick loop, err: %v", err)
		return
	}
}

func (s *Session) HandlePacket(from, to mapper.Protocol, name string, head, data []byte) ([]byte, error) {
	// 要做修改的包
	switch name {
	case "GetPlayerTokenReq":
		return s.OnGetPlayerTokenReq(from, to, data)
	case "GetPlayerTokenRsp":
		return s.OnGetPlayerTokenRsp(from, to, data)
	case "UnionCmdNotify":
		return s.OnUnionCmdNotify(from, to, data)
	case "ClientAbilityChangeNotify":
		return s.OnClientAbilityChangeNotify(from, to, data)
	case "ClientAbilityInitFinishNotify":
		return s.OnClientAbilityInitFinishNotify(from, to, data)
	case "AbilityInvocationsNotify":
		return s.OnAbilityInvocationsNotify(from, to, data)
	case "CombatInvocationsNotify":
		return s.OnCombatInvocationsNotify(from, to, data)
	case "ClientSetGameTimeReq":
		return s.OnClientSetGameTimeReq(from, to, head, data)
	case "ChangeGameTimeRsp":
		return s.OnChangeGameTimeRsp(from, to, head, data)
	}
	if s.config.Console.Enabled {
		switch name {
		case "GetPlayerFriendListRsp":
			return s.OnGetPlayerFriendListRsp(from, to, data)
		case "PrivateChatReq":
			return s.OnPrivateChatReq(from, to, head, data)
		case "PullPrivateChatReq":
			return s.OnPullPrivateChatReq(from, to, data)
		case "PullRecentChatReq":
			return s.OnPullRecentChatReq(from, to, data)
		case "PullRecentChatRsp":
			return s.OnPullRecentChatRsp(from, to, data)
		case "MarkMapReq":
			return s.OnMarkMapReq(from, to, head, data)
		}
	}
	// 不做修改的包
	switch name {
	case "PlayerEnterSceneNotify":
		s.HandlePlayerEnterSceneNotify(data)
	case "PostEnterSceneRsp":
		if s.playerSceneId != s.playerPrevSceneId {
			logger.Debug("player jump scene, old: %v, new: %v, uid: %v", s.playerPrevSceneId, s.playerSceneId, s.playerUid)
			for _, shellCode := range LuaShellCode {
				s.SendLuaShellCode(shellCode)
			}
		}
	}
	if config.GetConfig().TerrainCollect {
		switch name {
		case "EntityMoveInfo":
			s.HandleEntityMoveInfo(data)
		}
	}
	return data, nil
}

type UnionCmdNotify struct {
	CmdList []*UnionCmd `json:"cmdList"`
}

type UnionCmd struct {
	MessageID uint16 `json:"messageId"`
	Body      []byte `json:"body"`
}

func (s *Session) OnUnionCmdNotify(from, to mapper.Protocol, data []byte) ([]byte, error) {
	notify := new(UnionCmdNotify)
	err := json.Unmarshal(data, notify)
	if err != nil {
		return data, err
	}
	for _, cmd := range notify.CmdList {
		name := s.mapping.CommandNameMap[from][cmd.MessageID]
		cmd.MessageID = s.mapping.CommandPairMap[from][to][cmd.MessageID]
		cmd.Body, err = s.ConvertPacketByName(from, to, name, cmd.Body)
		if err != nil {
			return data, err
		}
	}
	return json.Marshal(notify)
}

type PlayerEnterSceneNotify struct {
	SceneId     uint32 `json:"sceneId"`
	PrevSceneId uint32 `json:"prevSceneId"`
}

func (s *Session) HandlePlayerEnterSceneNotify(data []byte) {
	ntf := new(PlayerEnterSceneNotify)
	err := json.Unmarshal(data, ntf)
	if err != nil {
		// 解析失败
		return
	}
	s.playerSceneId = ntf.SceneId
	s.playerPrevSceneId = ntf.PrevSceneId
}

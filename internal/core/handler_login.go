package core

import (
	"encoding/binary"
	"encoding/json"

	"github.com/Jx2f/ViaGenshin/internal/mapper"
	"github.com/Jx2f/ViaGenshin/pkg/crypto/mt19937"
	"github.com/Jx2f/ViaGenshin/pkg/logger"
)

type GetPlayerTokenReq struct {
	KeyID         uint32 `json:"keyId,omitempty"`
	ClientRandKey string `json:"clientRandKey,omitempty"`
}

func (s *Session) OnGetPlayerTokenReq(from, to mapper.Protocol, data []byte) ([]byte, error) {
	logger.Debug("[DebugPacketLog] uid: %v, srcPbVer: %v, dstPbVer: %v, cmdName: %v, pkt: %v",
		s.playerUid, from, to, "GetPlayerTokenReq", string(data))
	packet := new(GetPlayerTokenReq)
	err := json.Unmarshal(data, &packet)
	if err != nil {
		return data, err
	}
	seed, err := s.keys.ServerKey.DecryptBase64(packet.ClientRandKey)
	if err != nil {
		return data, err
	}
	s.loginRand = binary.BigEndian.Uint64(seed)
	return data, nil
}

type GetPlayerTokenRsp struct {
	RetCode       int32  `json:"retcode,omitempty"`
	Uid           uint32 `json:"uid,omitempty"`
	KeyID         uint32 `json:"keyId,omitempty"`
	ServerRandKey string `json:"serverRandKey,omitempty"`
}

func (s *Session) OnGetPlayerTokenRsp(from, to mapper.Protocol, data []byte) ([]byte, error) {
	logger.Debug("[DebugPacketLog] uid: %v, srcPbVer: %v, dstPbVer: %v, cmdName: %v, pkt: %v",
		s.playerUid, from, to, "GetPlayerTokenRsp", string(data))
	packet := new(GetPlayerTokenRsp)
	err := json.Unmarshal(data, &packet)
	if err != nil {
		return data, err
	}
	s.playerUid = packet.Uid
	if packet.RetCode != 0 {
		return data, nil
	}
	seed, err := s.keys.ClientKeys[packet.KeyID].DecryptBase64(packet.ServerRandKey)
	if err != nil {
		return data, err
	}
	s.loginKey = mt19937.NewKeyBlock(s.loginRand ^ binary.BigEndian.Uint64(seed))
	return data, nil
}

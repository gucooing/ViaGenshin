package core

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/Jx2f/ViaGenshin/internal/mapper"
	"github.com/Jx2f/ViaGenshin/pkg/logger"
	"github.com/Jx2f/ViaGenshin/pkg/transport/kcp"
)

type Engine struct {
	cachedPullRecentChat    *PullRecentChatReq
	cachedClientSetGameTime *ClientSetGameTimeReq
}

type SystemHint struct {
	Type int32 `json:"type,omitempty"`
}

type ChatInfo struct {
	Time       uint32      `json:"time,omitempty"`
	Sequence   uint32      `json:"sequence,omitempty"`
	ToUid      uint32      `json:"toUid,omitempty"`
	Uid        uint32      `json:"uid,omitempty"`
	IsRead     bool        `json:"isRead,omitempty"`
	Text       string      `json:"text,omitempty"`
	Icon       uint32      `json:"icon,omitempty"`
	SystemHint *SystemHint `json:"systemHint,omitempty"`
}

type PrivateChatNotify struct {
	ChatInfo *ChatInfo `json:"chatInfo,omitempty"`
}

func (s *Session) NotifyPrivateChat(toSession *kcp.Session, to mapper.Protocol, head []byte, chatInfo *ChatInfo) error {
	packet := new(PrivateChatNotify)
	packet.ChatInfo = chatInfo
	data, err := json.Marshal(packet)
	if err != nil {
		return err
	}
	logger.Debug("Injecting PrivateChatNotify: %s", data)
	return s.SendPacketJSON(toSession, to, "PrivateChatNotify", head, data)
}

type PrivateChatReq struct {
	TargetUid uint32 `json:"targetUid,omitempty"`
	Text      string `json:"text,omitempty"`
	Icon      uint32 `json:"icon,omitempty"`
}

type PrivateChatRsp struct {
	ChatForbiddenEndtime uint32 `json:"chatForbiddenEndtime,omitempty"`
	Retcode              int32  `json:"retcode,omitempty"`
}

func (s *Session) OnPrivateChatReq(from, to mapper.Protocol, head, data []byte) ([]byte, error) {
	in := new(PrivateChatReq)
	err := json.Unmarshal(data, &in)
	if err != nil {
		return data, err
	}
	if in.TargetUid != consoleUid {
		return data, nil
	}
	logger.Debug("Injecting PrivateChatReq: %s", data)
	if err = s.NotifyPrivateChat(s.endpoint, from, head, &ChatInfo{
		Time:  uint32(time.Now().Unix()),
		ToUid: consoleUid,
		Uid:   s.playerUid,
		Text:  in.Text,
		Icon:  in.Icon,
	}); err != nil {
		return data, err
	}
	if in.Text == "" {
		return data, nil
	}
	headTmp := make([]byte, len(head))
	copy(headTmp, head)
	go s.handleChatMsgAsync(in.Text, from, headTmp)
	out := new(PrivateChatRsp)
	p, err := json.Marshal(out)
	if err != nil {
		return data, err
	}
	logger.Debug("Injecting PrivateChatRsp: %s", data)
	if err = s.SendPacketJSON(s.endpoint, to, "PrivateChatRsp", head, p); err != nil {
		return data, err
	}
	return data, fmt.Errorf("injected PrivateChatReq")
}

// 异步非阻塞处理聊天消息
func (s *Session) handleChatMsgAsync(textInput string, from mapper.Protocol, head []byte) {
	// 发起http请求
	textOutput := s.ConsoleExecute(1116, s.playerUid, textInput)
	// 返回结果给客户端
	err := s.NotifyPrivateChat(s.endpoint, from, head, &ChatInfo{
		Time:  uint32(time.Now().Unix()),
		ToUid: s.playerUid,
		Uid:   consoleUid,
		Text:  textOutput,
	})
	if err != nil {
		logger.Error("send error: %v", err)
	}
}

type PullPrivateChatReq struct {
	TargetUid     uint32 `json:"targetUid,omitempty"`
	PullNum       uint32 `json:"pullNum,omitempty"`
	BeginSequence uint32 `json:"beginSequence,omitempty"`
}

type PullPrivateChatRsp struct {
	ChatInfo []*ChatInfo `json:"chatInfo,omitempty"`
	Retcode  int32       `json:"retcode,omitempty"`
}

func (s *Session) OnPullPrivateChatReq(from, to mapper.Protocol, data []byte) ([]byte, error) {
	in := new(PullPrivateChatReq)
	err := json.Unmarshal(data, &in)
	if err != nil {
		return data, err
	}
	if in.TargetUid != consoleUid {
		return data, nil
	}
	logger.Debug("Injecting PullPrivateChatReq: %s", data)
	out := new(PullPrivateChatRsp)
	err = json.Unmarshal(data, &out)
	if err != nil {
		return data, err
	}
	out.ChatInfo = append(out.ChatInfo, &ChatInfo{
		Time:  uint32(time.Now().Unix()),
		ToUid: s.playerUid,
		Uid:   consoleUid,
		Text:  consoleWelcomeText,
	})
	out.Retcode = 0
	p, err := json.Marshal(out)
	if err != nil {
		return data, err
	}
	logger.Debug("Injecting PullPrivateChatRsp: %s", p)
	return data, fmt.Errorf("injected PullPrivateChatReq")
}

type PullRecentChatReq struct {
	PullNum       uint32 `json:"pullNum,omitempty"`
	BeginSequence uint32 `json:"beginSequence,omitempty"`
}

func (s *Session) OnPullRecentChatReq(from, to mapper.Protocol, data []byte) ([]byte, error) {
	packet := new(PullRecentChatReq)
	err := json.Unmarshal(data, &packet)
	if err != nil {
		return data, err
	}
	if packet.BeginSequence != 0 {
		return data, nil
	}
	s.cachedPullRecentChat = packet
	logger.Debug("Injecting PullRecentChatReq: %s", data)
	return data, nil
}

type PullRecentChatRsp struct {
	ChatInfo []*ChatInfo `json:"chatInfo,omitempty"`
	Retcode  int32       `json:"retcode,omitempty"`
}

func (s *Session) OnPullRecentChatRsp(from, to mapper.Protocol, data []byte) ([]byte, error) {
	if s.cachedPullRecentChat == nil || s.cachedPullRecentChat.BeginSequence != 0 {
		return data, nil
	}
	s.cachedPullRecentChat = nil
	packet := new(PullRecentChatRsp)
	err := json.Unmarshal(data, &packet)
	if err != nil {
		return data, err
	}
	packet.ChatInfo = append(packet.ChatInfo, &ChatInfo{
		Time:  uint32(time.Now().Unix()),
		ToUid: s.playerUid,
		Uid:   consoleUid,
		Text:  consoleWelcomeText,
	})
	packet.Retcode = 0
	data, err = json.Marshal(packet)
	if err != nil {
		return data, err
	}
	logger.Debug("Injecting PullRecentChatRsp: %s", data)
	return data, nil
}

type GetPlayerFriendListRsp struct {
	Retcode       int32             `json:"retcode,omitempty"`
	AskFriendList []*map[string]any `json:"askFriendList,omitempty"`
	FriendList    []*map[string]any `json:"friendList,omitempty"`
}

func (s *Session) OnGetPlayerFriendListRsp(from, to mapper.Protocol, data []byte) ([]byte, error) {
	packet := new(GetPlayerFriendListRsp)
	err := json.Unmarshal(data, &packet)
	if err != nil {
		return data, err
	}
	packet.FriendList = append(packet.FriendList, &map[string]any{
		"uid":        consoleUid,
		"nickname":   consoleNickname,
		"level":      consoleLevel,
		"worldLevel": consoleWorldLevel,
		"signature":  consoleSignature,
		"nameCardId": consoleNameCardId,
		"profilePicture": map[string]any{
			"avatarId":  consoleAvatarId,
			"costumeId": consoleCostumeId,
		},
		"isGameSource": true,
		"onlineState":  uint32(1),
		"platformType": uint32(3),
	})
	data, err = json.Marshal(packet)
	if err != nil {
		return data, err
	}
	logger.Debug("Injecting GetPlayerFriendListRsp: %s", data)
	return data, nil
}

type Vector struct {
	X float32 `json:"x,omitempty"`
	Y float32 `json:"y,omitempty"`
	Z float32 `json:"z,omitempty"`
}

type MapMarkPoint struct {
	SceneID   uint32  `json:"sceneId,omitempty"`
	Name      string  `json:"name,omitempty"`
	Pos       *Vector `json:"pos,omitempty"`
	PointType int32   `json:"pointType,omitempty"`
	MonsterID uint32  `json:"monsterId,omitempty"`
	FromType  int32   `json:"fromType,omitempty"`
	QuestID   uint32  `json:"questId,omitempty"`
}

type MarkMapReq struct {
	Op   int32         `json:"op,omitempty"`
	Old  *MapMarkPoint `json:"old,omitempty"`
	Mark *MapMarkPoint `json:"mark,omitempty"`
}

func (s *Session) OnMarkMapReq(from, to mapper.Protocol, head, data []byte) ([]byte, error) {
	packet := new(MarkMapReq)
	err := json.Unmarshal(data, &packet)
	if err != nil {
		return data, err
	}
	if !(packet.Mark != nil && packet.Mark.Name == "goto" && packet.Mark.Pos != nil) {
		return data, nil
	}
	if packet.Mark.Pos.Y == 0 {
		packet.Mark.Pos.Y = 500
	}
	logger.Debug("Injecting MarkMapReq: %s", data)
	s.ConsoleExecute(1116, s.playerUid, fmt.Sprintf("goto %f %f %f", packet.Mark.Pos.X, packet.Mark.Pos.Y, packet.Mark.Pos.Z))
	return data, fmt.Errorf("injected MarkMapReq")
}

type ChangeGameTimeReq struct {
	IsForceSet bool   `json:"isForceSet,omitempty"`
	GameTime   uint32 `json:"gameTime,omitempty"`
	ExtraDays  uint32 `json:"extraDays,omitempty"`
}

type ClientSetGameTimeReq struct {
	IsForceSet     bool   `json:"isForceSet,omitempty"`
	GameTime       uint32 `json:"gameTime,omitempty"`
	ClientGameTime uint32 `json:"clientGameTime,omitempty"`
}

func (s *Session) OnClientSetGameTimeReq(from, to mapper.Protocol, head, data []byte) ([]byte, error) {
	in := new(ClientSetGameTimeReq)
	err := json.Unmarshal(data, &in)
	if err != nil {
		return data, err
	}
	s.cachedClientSetGameTime = in
	out := new(ChangeGameTimeReq)
	out.IsForceSet = in.IsForceSet
	out.GameTime = in.GameTime % 1440
	out.ExtraDays = (in.GameTime - in.ClientGameTime) / 1440
	p, err := json.Marshal(out)
	if err != nil {
		return data, err
	}
	logger.Debug("Rewriting %s: ClientSetGameTimeReq to %s:ChangeGameTimeReq, from: %v, to: %v", from, to, data, p)
	err = s.SendPacketJSON(s.upstream, to, "ChangeGameTimeReq", head, p)
	if err != nil {
		return data, err
	}
	return data, fmt.Errorf("injected ChangeGameTimeReq")
}

type ChangeGameTimeRsp struct {
	Retcode   int32  `json:"retcode,omitempty"`
	GameTime  uint32 `json:"gameTime,omitempty"`
	ExtraDays uint32 `json:"extraDays,omitempty"`
}

type ClientSetGameTimeRsp struct {
	Retcode        int32  `json:"retcode,omitempty"`
	GameTime       uint32 `json:"gameTime,omitempty"`
	ClientGameTime uint32 `json:"clientGameTime,omitempty"`
}

func (s *Session) OnChangeGameTimeRsp(from, to mapper.Protocol, head, data []byte) ([]byte, error) {
	if s.cachedClientSetGameTime == nil {
		return data, nil
	}
	in := new(ChangeGameTimeRsp)
	err := json.Unmarshal(data, &in)
	if err != nil {
		return data, err
	}
	logger.Debug("Injecting ChangeGameTimeRsp: %s", data)
	out := new(ClientSetGameTimeRsp)
	out.GameTime = s.cachedClientSetGameTime.GameTime
	out.ClientGameTime = s.cachedClientSetGameTime.ClientGameTime
	s.cachedClientSetGameTime = nil
	p, err := json.Marshal(out)
	if err != nil {
		return data, err
	}
	logger.Debug("Rewriting %s: ChangeGameTimeRsp to %s:ClientSetGameTimeRsp, from: %v, to: %v", from, to, data, p)
	err = s.SendPacketJSON(s.endpoint, to, "ClientSetGameTimeRsp", head, p)
	if err != nil {
		return data, err
	}
	return data, fmt.Errorf("injected ClientSetGameTimeRsp")
}

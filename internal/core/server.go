package core

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jhump/protoreflect/dynamic"

	"github.com/Jx2f/ViaGenshin/internal/config"
	"github.com/Jx2f/ViaGenshin/internal/mapper"
	"github.com/Jx2f/ViaGenshin/pkg/crypto/mt19937"
	"github.com/Jx2f/ViaGenshin/pkg/logger"
	"github.com/Jx2f/ViaGenshin/pkg/transport"
	"github.com/Jx2f/ViaGenshin/pkg/transport/kcp"
)

var CLIENT_CONN_NUM int32

type Server struct {
	*Service
	config *config.ConfigEndpoints

	mu       sync.RWMutex
	protocol mapper.Protocol
	listener *kcp.Listener
	sessions map[uint32]*Session
}

func NewServer(s *Service, c *config.ConfigEndpoints, v config.Protocol) (*Server, error) {
	e := new(Server)
	e.Service = s
	e.config = c
	var err error
	e.protocol = v
	e.listener, err = kcp.Listen(e.config.Mapping[v])
	if err != nil {
		return nil, err
	}
	e.sessions = make(map[uint32]*Session)
	return e, nil
}

func (s *Server) Start(ctx context.Context) error {
	logger.Info("Start listening on %s", s.listener.Addr())
	go s.printNetInfo()
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		conn, err := s.listener.Accept()
		if err != nil {
			return err
		}
		go s.handleConn(conn)
	}
}

var (
	KCP_SEND_BPS uint64
	KCP_RECV_BPS uint64
	UDP_SEND_BPS uint64
	UDP_RECV_BPS uint64
	UDP_SEND_PPS uint64
	UDP_RECV_PPS uint64
)

func (s *Server) printNetInfo() {
	ticker := time.NewTicker(time.Second * 60)
	for {
		<-ticker.C
		snmp := kcp.DefaultSnmp.Copy()
		KCP_SEND_BPS = snmp.BytesSent / 60
		KCP_RECV_BPS = snmp.BytesReceived / 60
		UDP_SEND_BPS = snmp.OutBytes / 60
		UDP_RECV_BPS = snmp.InBytes / 60
		UDP_SEND_PPS = snmp.OutPkts / 60
		UDP_RECV_PPS = snmp.InPkts / 60
		logger.Info("kcp send: %v B/s, kcp recv: %v B/s", KCP_SEND_BPS, KCP_RECV_BPS)
		logger.Info("udp send: %v B/s, udp recv: %v B/s", UDP_SEND_BPS, UDP_RECV_BPS)
		logger.Info("udp send: %v pps, udp recv: %v pps", UDP_SEND_PPS, UDP_RECV_PPS)
		clientConnNum := atomic.LoadInt32(&CLIENT_CONN_NUM)
		logger.Info("client conn num: %v", clientConnNum)
		kcp.DefaultSnmp.Reset()
	}
}

func (s *Server) handleConn(conn *kcp.Session) {
	logger.Info("New session from %s", conn.RemoteAddr())
	if err := s.NewSession(conn).Start(); err != nil {
		logger.Error("Session %d closed, err: %v", conn.SessionID(), err)
		return
	}
}

func (s *Server) NewSession(conn *kcp.Session) *Session {
	s.mu.Lock()
	defer s.mu.Unlock()
	session := newSession(s, conn)
	s.sessions[conn.SessionID()] = session
	return session
}

type Session struct {
	*Server
	endpoint *kcp.Session
	upstream *kcp.Session

	loginRand         uint64
	loginKey          *mt19937.KeyBlock
	playerUid         uint32
	playerSceneId     uint32
	playerPrevSceneId uint32

	Engine
}

func newSession(s *Server, endpoint *kcp.Session) *Session {
	return &Session{Server: s, endpoint: endpoint}
}

func (s *Session) Start() error {
	var err error
	s.upstream, err = kcp.Dial(s.config.MainEndpoint)
	if err != nil {
		return err
	}
	logger.Info("Start forwarding session %d to %s, mapping %s <-> %s", s.endpoint.SessionID(), s.upstream.RemoteAddr(), s.protocol, s.config.MainProtocol)
	return s.Forward()
}

func (s *Session) Forward() error {
	atomic.AddInt32(&CLIENT_CONN_NUM, 1)
	wg := new(sync.WaitGroup)
	wg.Add(2)
	go func() {
		defer wg.Done()
		recvBuf := make([]byte, 343*1024)
		for {
			n, err := s.endpoint.UpdateRecv(recvBuf)
			if err != nil {
				logger.Warn("exit endpoint recv loop, err: %v, id: %v", err, s.endpoint.SessionID())
				break
			}
			if n == 0 {
				time.Sleep(time.Millisecond * 10)
				continue
			}
			payload := recvBuf[:n]
			if err := s.ConvertPayload(
				s.endpoint, s.upstream, s.protocol, s.config.MainProtocol, payload,
			); err != nil {
				logger.Warn("Failed to convert endpoint payload, err: %v", err)
			}
		}
		s.endpoint.LogicClose()
	}()
	go func() {
		defer wg.Done()
		recvBuf := make([]byte, 343*1024)
		for {
			n, err := s.upstream.UpdateRecv(recvBuf)
			if err != nil {
				logger.Warn("exit upstream recv loop, err: %v, id: %v", err, s.upstream.SessionID())
				break
			}
			if n == 0 {
				time.Sleep(time.Millisecond * 10)
				continue
			}
			payload := recvBuf[:n]
			if err := s.ConvertPayload(
				s.upstream, s.endpoint, s.config.MainProtocol, s.protocol, payload,
			); err != nil {
				logger.Warn("Failed to convert upstream payload, err: %v", err)
			}
		}
		s.upstream.LogicClose()
		s.endpoint.LogicClose()
		err := s.listener.DisconnectSession(s.endpoint, kcp.DisconnectReason(s.upstream.GetCloseReason()))
		if err != nil {
			logger.Error("error: %v", err)
		}
	}()
	wg.Wait()
	atomic.AddInt32(&CLIENT_CONN_NUM, -1)
	return nil
}

func (s *Session) ConvertPayload(
	fromSession, toSession *kcp.Session,
	from, to mapper.Protocol, payload transport.Payload,
) error {
	n := len(payload)
	if n < 12 {
		return errors.New("packet too short")
	}
	if err := s.EncryptPayload(payload, false); err != nil {
		return err
	}
	if payload[0] != 0x45 || payload[1] != 0x67 || payload[n-2] != 0x89 || payload[n-1] != 0xAB {
		return errors.New("invalid payload")
	}
	b := bytes.NewBuffer(payload[2 : n-2])
	fromCmd := binary.BigEndian.Uint16(b.Next(2))
	n1 := binary.BigEndian.Uint16(b.Next(2))
	n2 := binary.BigEndian.Uint32(b.Next(4))
	if uint32(n) != 12+uint32(n1)+n2 {
		return errors.New("invalid packet length")
	}
	head := b.Next(int(n1))
	fromData := b.Next(int(n2))
	toCmd := fromCmd
	if from != to {
		toCmd = s.mapping.CommandPairMap[from][to][fromCmd]
	}
	toData, err := s.ConvertPacket(from, to, fromCmd, head, fromData)
	if err != nil {
		return err
	}
	return s.SendPacket(toSession, to, toCmd, head, toData)
}

func (s *Session) EncryptPayload(payload transport.Payload, first bool) error {
	n := len(payload)
	if n < 4 {
		return errors.New("packet too short")
	}
	var encrypt = payload[0] == 0x45 && payload[1] == 0x67 && payload[n-2] == 0x89 && payload[n-1] == 0xAB
	if s.loginKey != nil && !first {
		s.loginKey.Xor(payload)
		if !encrypt && (payload[0] != 0x45 || payload[1] != 0x67 || payload[n-2] != 0x89 || payload[n-1] != 0xAB) {
			// revert
			s.loginKey.Xor(payload)
		} else {
			return nil
		}
	}
	s.keys.SharedKey.Xor(payload)
	return nil
}

func (s *Session) SendPacket(toSession *kcp.Session, to mapper.Protocol, toCmd uint16, toHead, toData []byte) error {
	b := bytes.NewBuffer(nil)
	b.Write([]byte{0x45, 0x67})
	binary.Write(b, binary.BigEndian, toCmd)
	binary.Write(b, binary.BigEndian, uint16(len(toHead)))
	binary.Write(b, binary.BigEndian, uint32(len(toData)))
	b.Write(toHead)
	b.Write(toData)
	b.Write([]byte{0x89, 0xAB})
	payload := b.Bytes()
	name := s.mapping.CommandNameMap[to][toCmd]
	if err := s.EncryptPayload(payload, name == "GetPlayerTokenReq" || name == "GetPlayerTokenRsp"); err != nil {
		return err
	}
	return toSession.SendPayload(payload)
}

func (s *Session) SendPacketJSON(toSession *kcp.Session, to mapper.Protocol, name string, toHead, data []byte) error {
	toCmd := s.mapping.BaseCommands[name]
	if s.mapping.BaseProtocol != to {
		if toCmd == 0 {
			for k, v := range s.mapping.CommandNameMap[to] {
				if v == name {
					toCmd = k
					break
				}
			}
		} else {
			toCmd = s.mapping.CommandPairMap[s.mapping.BaseProtocol][to][toCmd]
		}
	}
	toDesc := s.mapping.MessageDescMap[to][name]
	if toDesc == nil {
		return fmt.Errorf("unknown to message %s in %s", name, to)
	}
	toPacket := dynamic.NewMessage(toDesc)
	if err := toPacket.UnmarshalJSONPB(UnmarshalOptions, data); err != nil {
		return err
	}
	toData, err := toPacket.Marshal()
	if err != nil {
		return err
	}
	logger.Debug("Sending packet %s(%d) to %s, to: %v", name, toCmd, to, string(data))
	return s.SendPacket(toSession, to, toCmd, toHead, toData)
}

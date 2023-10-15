package core

import (
	"fmt"
	"strings"

	"github.com/Jx2f/ViaGenshin/internal/config"
	"github.com/Jx2f/ViaGenshin/pkg/logger"
	"github.com/golang/protobuf/jsonpb"
	"github.com/jhump/protoreflect/dynamic"

	"github.com/Jx2f/ViaGenshin/internal/mapper"
)

var (
	MarshalOptions = &jsonpb.Marshaler{
		EnumsAsInts: true,
	}
	UnmarshalOptions = &jsonpb.Unmarshaler{
		AllowUnknownFields: true,
	}
)

func (s *Session) ConvertPacket(from, to mapper.Protocol, fromCmd uint16, head, p []byte) ([]byte, error) {
	name := s.mapping.CommandNameMap[from][fromCmd]
	fromDesc := s.mapping.MessageDescMap[from][name]
	if fromDesc == nil {
		return p, fmt.Errorf("unknown from message %s(%d) in %s", name, fromCmd, from)
	}
	fromPacket := dynamic.NewMessage(fromDesc)
	if err := fromPacket.Unmarshal(p); err != nil {
		return p, err
	}
	fromJson, err := fromPacket.MarshalJSONPB(MarshalOptions)
	if err != nil {
		return p, err
	}
	toJson, err := s.HandlePacket(from, to, name, head, fromJson)
	if err != nil {
		if strings.HasPrefix(err.Error(), "injected ") {
			return p, nil
		}
		return p, err
	}
	if s.playerUid == config.GetConfig().DebugPacketLogUid {
		logger.Debug("[DebugPacketLog] uid: %v, srcPbVer: %v, dstPbVer: %v, cmdName: %v, srcPkt: %v, dstPkt: %v",
			s.playerUid, from, to, name, string(fromJson), string(toJson))
	}
	toDesc := s.mapping.MessageDescMap[to][name]
	if toDesc == nil {
		return p, fmt.Errorf("unknown to message %s in %s", name, to)
	}
	toPacket := dynamic.NewMessage(toDesc)
	if err := toPacket.UnmarshalJSONPB(UnmarshalOptions, toJson); err != nil {
		return p, err
	}
	toJson, err = toPacket.MarshalJSONPB(MarshalOptions)
	if err != nil {
		return p, err
	}
	return toPacket.Marshal()
}

func (s *Session) ConvertPacketByName(from, to mapper.Protocol, name string, p []byte) ([]byte, error) {
	fromDesc := s.mapping.MessageDescMap[from][name]
	if fromDesc == nil {
		return p, fmt.Errorf("unknown from message %s in %s", name, from)
	}
	fromPacket := dynamic.NewMessage(fromDesc)
	if err := fromPacket.Unmarshal(p); err != nil {
		return p, err
	}
	fromJson, err := fromPacket.MarshalJSONPB(MarshalOptions)
	if err != nil {
		return p, err
	}
	toJson, err := s.HandlePacket(from, to, name, nil, fromJson)
	if err != nil {
		return p, err
	}
	if s.playerUid == config.GetConfig().DebugPacketLogUid {
		logger.Debug("[DebugPacketLog] uid: %v, srcPbVer: %v, dstPbVer: %v, cmdName: %v, srcPkt: %v, dstPkt: %v",
			s.playerUid, from, to, name, string(fromJson), string(toJson))
	}
	toDesc := s.mapping.MessageDescMap[to][name]
	if toDesc == nil {
		return p, fmt.Errorf("unknown to message %s in %s", name, to)
	}
	toPacket := dynamic.NewMessage(toDesc)
	if err := toPacket.UnmarshalJSONPB(UnmarshalOptions, toJson); err != nil {
		return p, err
	}
	toJson, err = toPacket.MarshalJSONPB(MarshalOptions)
	if err != nil {
		return p, err
	}
	return toPacket.Marshal()
}

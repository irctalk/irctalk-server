package common

import "reflect"

type ZmqPacket interface {
	GetPacketCommand() string
}

type ZmqChat struct {
	Log *IRCLog
}

func (z ZmqChat) GetPacketCommand() string {
	return "CHAT"
}

type ZmqAddChannel struct {
	Channel *IRCChannel
}

func (z ZmqAddChannel) GetPacketCommand() string {
	return "ADD_CHANNEL"
}

type ZmqServerStatus struct {
	Active bool
}

func (z ZmqServerStatus) GetPacketCommand() string {
	return "SERVER_STATUS"
}

type ZmqAddServer struct {
	ServerInfo *IRCServer
}

func (z ZmqAddServer) GetPacketCommand() string {
	return "ADD_SERVER"
}

type ZmqSendChat struct {
	Target, Message string
}

func (z ZmqSendChat) GetPacketCommand() string {
	return "SEND_CHAT"
}

type ZmqUpdateChannel struct {
	DeltaChannels []IRCDeltaChannel
}

func (z ZmqUpdateChannel) GetPacketCommand() string {
	return "UPDATE_CHANNEL"
}

type ZmqJoinPartChannel struct {
	Channel string
	Join bool
}

func (z ZmqJoinPartChannel) GetPacketCommand() string {
	return "JOIN_PART_CHANNEL"
}

type ZmqDelChannel struct {
	Channel string
}

func (z ZmqDelChannel) GetPacketCommand() string {
	return "DEL_CHANNEL"
}

func RegisterPacket() {
	typeMap = make(map[string]reflect.Type)
	registerPacketType(ZmqChat{})
	registerPacketType(ZmqAddChannel{})
	registerPacketType(ZmqServerStatus{})
	registerPacketType(ZmqAddServer{})
	registerPacketType(ZmqSendChat{})
	registerPacketType(ZmqUpdateChannel{})
	registerPacketType(ZmqJoinPartChannel{})
	registerPacketType(ZmqDelChannel{})
}

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

func RegisterPacket() {
	typeMap = make(map[string]reflect.Type)
	registerPacketType(ZmqChat{})
	registerPacketType(ZmqAddChannel{})
	registerPacketType(ZmqServerStatus{})
	registerPacketType(ZmqAddServer{})
	registerPacketType(ZmqSendChat{})
}

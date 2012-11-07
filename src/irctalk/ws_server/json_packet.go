package main

import (
	"irctalk/common"
	"reflect"
	"log"
)

var reqTypeMap map[string]reflect.Type
var resTypeMap map[string]reflect.Type

type Packet struct {
	Cmd     string `json:"type"`
	MsgId   int    `json:"msg_id,omitempty"`
	Status  int    `json:"status"`
	Msg     string `json:"msg,omitempty"`
	RawData JSONData `json:"data"`
	body    interface{}
}

type JSONData []byte

func (p *JSONData) MarshalJSON() ([]byte, error) {
	return *p, nil
}

func (p *JSONData) UnmarshalJSON(data []byte) error {
	*p = data
	return nil
}

func (p Packet) MakeResponse() *Packet {
	t, ok := resTypeMap[p.Cmd]
	if !ok {
		log.Println("MakeResponse Error: ", p.Cmd)
		return nil
	}
	resp := reflect.New(t).Interface().(PacketCommand)
	return &Packet{
		Cmd:   resp.GetPacketCommand(),
		MsgId: p.MsgId,
		body:  resp,
	}
}

func MakePacket(p PacketCommand) *Packet {
	return &Packet{
		Cmd:  p.GetPacketCommand(),
		body: p,
	}
}

type PacketCommand interface {
	GetPacketCommand() string
}

func registerRequestPacketType(p PacketCommand) {
	t := reflect.TypeOf(p)
	reqTypeMap[p.GetPacketCommand()] = t
}

func registerResponsePacketType(p PacketCommand) {
	t := reflect.TypeOf(p)
	resTypeMap[p.GetPacketCommand()] = t
}

func RegisterPacket() {
	reqTypeMap = make(map[string]reflect.Type)
	resTypeMap = make(map[string]reflect.Type)
	registerRequestPacketType(ReqRegister{})
	registerRequestPacketType(ReqLogin{})
	registerRequestPacketType(ReqGetServers{})
	registerRequestPacketType(ReqGetInitLogs{})
	registerRequestPacketType(ReqGetPastLogs{})
	registerRequestPacketType(ReqSendLog{})
	registerRequestPacketType(ReqAddServer{})
	registerRequestPacketType(ReqAddChannel{})
	registerRequestPacketType(ReqJoinPartChannel{})
	registerRequestPacketType(ReqDelServer{})
	registerRequestPacketType(ReqDelChannel{})
	registerRequestPacketType(ReqEditChannel{})
	registerRequestPacketType(AckPushLog{})

	registerResponsePacketType(ResRegister{})
	registerResponsePacketType(ResLogin{})
	registerResponsePacketType(ResGetServers{})
	registerResponsePacketType(ResGetInitLogs{})
	registerResponsePacketType(ResGetPastLogs{})
	registerResponsePacketType(ResSendLog{})
	registerResponsePacketType(ResAddServer{})
	registerResponsePacketType(ResAddChannel{})
	registerResponsePacketType(ResJoinPartChannel{})
	registerResponsePacketType(ResDelChannel{})
	registerResponsePacketType(ResDelServer{})
	registerResponsePacketType(ResEditChannel{})
	registerResponsePacketType(SendPushLog{})
	registerResponsePacketType(SendServerActive{})
}

type ReqRegister struct {
	AccessToken string `json:"access_token"`
}

func (p ReqRegister) GetPacketCommand() string {
	return "register"
}

type ResRegister struct {
	AuthKey string `json:"auth_key"`
}

func (p ResRegister) GetPacketCommand() string {
	return "register"
}

type ReqLogin struct {
	AuthKey string `json:"auth_key"`
}

func (p ReqLogin) GetPacketCommand() string {
	return "login"
}

type ResLogin struct {
}

func (p ResLogin) GetPacketCommand() string {
	return "login"
}

type ReqGetServers struct {
}

func (p ReqGetServers) GetPacketCommand() string {
	return "getServers"
}

type ResGetServers struct {
	Servers  []*common.IRCServer  `json:"servers"`
	Channels []*common.IRCChannel `json:"channels"`
}

func (p ResGetServers) GetPacketCommand() string {
	return "getServers"
}

type ReqGetInitLogs struct {
	LastLogId int64 `json:"last_log_id"`
	LogCount  int   `json:"log_count"`
}

func (p ReqGetInitLogs) GetPacketCommand() string {
	return "getInitLogs"
}

type ResGetInitLogs struct {
	Logs []*common.IRCLog `json:"logs"`
}

func (p ResGetInitLogs) GetPacketCommand() string {
	return "getInitLogs"
}

type ReqGetPastLogs struct {
	ServerId  int    `json:"server_id"`
	Channel   string `json:"channel"`
	LastLogId int64  `json:"last_log_id"`
	LogCount  int    `json:"log_count"`
}

func (p ReqGetPastLogs) GetPacketCommand() string {
	return "getPastLogs"
}

type ResGetPastLogs struct {
	Logs []*common.IRCLog `json:"logs"`
}

func (p ResGetPastLogs) GetPacketCommand() string {
	return "getPastLogs"
}

type ReqSendLog struct {
	ServerId int    `json:"server_id"`
	Channel  string `json:"channel"`
	Message  string `json:"message"`
}

func (p ReqSendLog) GetPacketCommand() string {
	return "sendLog"
}

type ResSendLog struct {
	Log *common.IRCLog `json:"log"`
}

func (p ResSendLog) GetPacketCommand() string {
	return "sendLog"
}

type SendPushLog struct {
	Log  *common.IRCLog `json:"log"`
	Noti bool           `json:"noti,omitempty"`
}

func (p SendPushLog) GetPacketCommand() string {
	return "pushLog"
}

type AckPushLog struct {
	LogId int64 `json:"log_id"`
}

func (p AckPushLog) GetPacketCommand() string {
	return "pushLog"
}

type ReqAddServer struct {
	Server *common.IRCServer `json:"server"`
}

func (p ReqAddServer) GetPacketCommand() string {
	return "addServer"
}

type ResAddServer struct {
	Server *common.IRCServer `json:"server"`
}

func (p ResAddServer) GetPacketCommand() string {
	return "addServer"
}

type SendServerActive struct {
	ServerId int  `json:"server_id"`
	Active   bool `json:"active"`
}

func (p SendServerActive) GetPacketCommand() string {
	return "serverActive"
}

type ReqAddChannel struct {
	ServerId int    `json:"server_id"`
	Channel  string `json:"channel"`
}

func (p ReqAddChannel) GetPacketCommand() string {
	return "addChannel"
}

type ResAddChannel struct {
	Channel *common.IRCChannel `json:"channel"`
}

func (p ResAddChannel) GetPacketCommand() string {
	return "addChannel"
}

type PushUpdateChannel struct {
	Channel *common.IRCChannel `json:"channel"`
}

func (p PushUpdateChannel) GetPacketCommand() string {
	return "updateChannel"
}

type ReqJoinPartChannel struct {
	ServerId int    `json:"server_id"`
	Channel  string `json:"channel"`
	Join     bool   `json:"join"`
}

func (p ReqJoinPartChannel) GetPacketCommand() string {
	return "joinPartChannel"
}

type ResJoinPartChannel struct {
	Channel *common.IRCChannel `json:"channel"`
}

func (p ResJoinPartChannel) GetPacketCommand() string {
	return "joinPartChannel"
}

type ReqDelChannel struct {
	ServerId int    `json:"server_id"`
	Channel  string `json:"channel"`
}

func (p ReqDelChannel) GetPacketCommand() string {
	return "delChannel"
}

type ResDelChannel struct {
}

func (p ResDelChannel) GetPacketCommand() string {
	return "delChannel"
}

type ReqDelServer struct {
	ServerId int `json:"server_id"`
}

func (p ReqDelServer) GetPacketCommand() string {
	return "delServer"
}

type ResDelServer struct {
}

func (p ResDelServer) GetPacketCommand() string {
	return "delServer"
}

type ReqEditChannel struct {
	Server *common.IRCServer `json:"server"`
}

func (p ReqEditChannel) GetPacketCommand() string {
	return "editChannel"
}

type ResEditChannel struct {
	Server *common.IRCServer `json:"server"`
}

func (p ResEditChannel) GetPacketCommand() string {
	return "editChannel"
}

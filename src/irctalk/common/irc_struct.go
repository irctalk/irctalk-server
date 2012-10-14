package common

import (
	"time"
	"encoding/json"
)

func UnixMilli(t time.Time) int64 {
	return t.UnixNano() / 1000000
}

func Import(raw interface{}, v interface{}) error {
	b, err := json.Marshal(raw)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

type IRCLog struct {
	Log_id    int64  `json:"log_id"`
	Timestamp int64  `json:"timestamp"`
	Server_id int    `json:"server_id"`
	Channel   string `json:"channel"`
	From      string `json:"from,omitempty"`
	Message   string `json:"message"`
}

type IRCChannel struct {
	Server_id int     `json:"server_id"`
	Name      string  `json:"channel"`
	Topic     string  `json:"topic"`
	UserCount int     `json:"user_count"`
	Last_log  *IRCLog `json:"last_log"`
}

type IRCUser struct {
	Nickname string `json:"nickname"`
	Realname string `json:"realname"`
}

type IRCServerInfo struct {
	Host string `json:"host"`
	Port int    `json:"port"`
	SSL  bool   `json:"ssl"`
}

type IRCServer struct {
	Id       int            `json:"id"`
	Name     string         `json:"name"`
	Server   *IRCServerInfo `json:"server"`
	User     *IRCUser       `json:"user"`
	Active	 bool			`json:"active"`
}

var TestUser = &IRCUser{
	Nickname: "Hyunggi",
	Realname: "Hyunggi",
}

var TestLog = &IRCLog{
	Log_id:    1,
	Timestamp: UnixMilli(time.Now()),
	Server_id: 0,
	Channel:   "#hackfair",
	From:      "Hyunggi",
	Message:   "모닝",
}

var TestChannel = &IRCChannel{
	Server_id: 0,
	Name:      "#hackfair",
	Topic:     "호옹이",
	UserCount: 3,
	Last_log:  TestLog,
}

var TestServer = &IRCServerInfo{
	Host: "irc.uriirc.org",
	Port: 16661,
	SSL:  true,
}

var TestServerInfo = &IRCServer{
	Id:       0,
	Name:     "UriIRC",
	Server:   TestServer,
	User:     TestUser,
}

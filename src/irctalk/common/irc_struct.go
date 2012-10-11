package common

import (
	"time"
)

func UnixMilli(t time.Time) int64 {
	return t.UnixNano() / 1000000
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
	Channel   string  `json:"channel"`
	Topic     string  `json:"topic"`
	UserCount int     `json:"user_count"`
	Last_log  *IRCLog `json:"last_log"`
}

type IRCUser struct {
	Nickname string `json:"nickname"`
	Realname string `json:"realname"`
}

type IRCServer struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

type IRCServerInfo struct {
	Id       int           `json:"id"`
	Name     string        `json:"name"`
	Server   *IRCServer    `json:"server"`
	User     *IRCUser      `json:"user"`
	Channels []*IRCChannel `json:"channels"`
}

var TestUser = &IRCUser{
	Nickname: "Hyunggi",
	Realname: "Hyunggi",
}

var TestLog = &IRCLog{
	Log_id:    1,
	Timestamp: UnixMilli(time.Now()),
	Server_id: 0,
	Channel:   "#test",
	From:      "Hyunggi",
	Message:   "모닝",
}

var TestChannel = &IRCChannel{
	Server_id: 0,
	Channel:   "#test",
	Topic:     "호옹이",
	UserCount: 3,
	Last_log:  TestLog,
}

var TestServer = &IRCServer{
	Host: "irc.uriirc.org",
	Port: 16661,
}

var TestServerInfo = &IRCServerInfo{
	Id:       0,
	Name:     "UriIRC",
	Server:   TestServer,
	User:     TestUser,
	Channels: []*IRCChannel{TestChannel},
}

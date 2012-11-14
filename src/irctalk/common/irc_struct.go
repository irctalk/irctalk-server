package common

import (
	"fmt"
	"strings"
	"time"
	"redigo/redis"
)

func UnixMilli(t time.Time) int64 {
	return t.UnixNano() / 1000000
}

type IRCLog struct {
	LogId     int64  `json:"log_id"`
	UserId    string `json:"-"`
	Timestamp int64  `json:"timestamp"`
	ServerId  int    `json:"server_id"`
	Channel   string `json:"channel"`
	From      string `json:"from,omitempty"`
	Message   string `json:"message"`
	Noti	  bool   `json:"noti,omitempty"`
}

type IRCChannel struct {
	UserId   string   `json:"-"`
	ServerId int      `json:"server_id"`
	Name     string   `json:"channel"`
	Topic    string   `json:"topic"`
	Members  []string `json:"members"`
	Joined   bool     `json:"joined"`
	LastLog  *IRCLog  `json:"last_log,omitempty"`
}

type IRCDeltaChannel map[string]interface{}

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
	UserId string         `json:"-"`
	Id     int            `json:"id"`
	Name   string         `json:"name"`
	Server *IRCServerInfo `json:"server"`
	User   *IRCUser       `json:"user"`
	Active bool           `json:"active"`
}

func (v *IRCChannel) GetKey() string {
	return fmt.Sprintf("channel:%s:%d:%s", v.UserId, v.ServerId, strings.ToLower(v.Name))
}

func (v *IRCServer) GetKey() string {
	return fmt.Sprintf("server:%s:%d", v.UserId, v.Id)
}

func (v *IRCLog) GetKey() string {
	return fmt.Sprintf("log:%s:%d:%s", v.UserId, v.ServerId, strings.ToLower(v.Channel))
}

func (v *IRCLog) RedisSave(r redis.Conn) error {
	data, err := GobEncode(v)
	if err != nil {
		return err
	}
	_, err = r.Do("ZADD", v.GetKey(), v.LogId, data)
	return err
}

func GetLastLogs(userId string, serverId int, channel string, lastLogId int64, count int) ([]*IRCLog, error) {
	r := pool.Get()
	defer r.Close()

	key := fmt.Sprintf("log:%s:%d:%s", userId, serverId, strings.ToLower(channel))

	if count == 0 {
		count = 30
	}
	var min string
	if lastLogId != -1 {
		min = "-inf"
	} else {
		min = fmt.Sprintf("(%d", lastLogId)
	}
	reply, err := redis.Values(r.Do("ZREVRANGEBYSCORE", key, "+inf", min, "LIMIT", 0, count))
	if err != nil {
		return nil, err
	}

	logs := make([]*IRCLog, len(reply))
	for i := 0; i < len(reply); i++ {
		data, err := redis.Bytes(reply[i], nil)
		if err != nil {
			return nil, err
		}
		var log IRCLog
		err = GobDecode(data, &log)
		if err != nil {
			return nil, err
		}
		logs[len(logs)-1-i] = &log
	}
	return logs, nil
}

func GetPastLogs(userId string, serverId int, channel string, lastLogId int64, count int) ([]*IRCLog, error) {
	r := pool.Get()
	defer r.Close()

	key := fmt.Sprintf("log:%s:%d:%s", userId, serverId, strings.ToLower(channel))
	max := fmt.Sprintf("(%d", lastLogId)

	if count == 0 {
		count = 30
	}

	reply, err := redis.Values(r.Do("ZREVRANGEBYSCORE", key, max, "-inf", "LIMIT", 0, count))
	if err != nil {
		return nil, err
	}

	logs := make([]*IRCLog, len(reply))
	for i := 0; i < len(reply); i++ {
		data, err := redis.Bytes(reply[i], nil)
		if err != nil {
			return nil, err
		}
		var log IRCLog
		err = GobDecode(data, &log)
		if err != nil {
			return nil, err
		}
		logs[len(logs)-1-i] = &log
	}
	return logs, nil
}

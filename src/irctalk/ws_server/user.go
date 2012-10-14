package main

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"irctalk/common"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Errors
type UserNotFound struct {
	id string
}

func (e *UserNotFound) Error() string {
	return fmt.Sprintf("User(%s) is not found!", e.id)
}

type CacheNotFound struct {
	key string
}

func (e *CacheNotFound) Error() string {
	return "Cache Not Found at Key : " + e.key
}

// UserManager
type UserManager struct {
	users      map[string]*User
	caches     map[string]string
	register   chan *Connection
	unregister chan *Connection
	broadcast  chan *UserMessage
}

func (um *UserManager) GetUserByKey(key string) (*User, error) {
	r := manager.redis.Get()
	defer manager.redis.Put(r)

	id, exist := r.Hget("key", key)
	if exist != nil {
		return nil, &CacheNotFound{key: key}
	}
	return um.GetUserById(string(id))
}

func (um *UserManager) GetUserById(id string) (*User, error) {
	user, ok := um.users[id]
	if !ok {
		return nil, &UserNotFound{id: id}
	}
	return user, nil
}

func (um *UserManager) NewUser(id string) *User {
	if _, exist := um.users[id]; exist {
		return nil
	}
	um.users[id] = &User{
		Id:    id,
		conns: make(map[*Connection]bool),
	}
	return um.users[id]
}

func (um *UserManager) RegisterUser(id string) string {
	// make key
	seed, _ := time.Now().GobEncode()
	h := hmac.New(sha1.New, seed)
	key := fmt.Sprintf("%0x", h.Sum([]byte(id)))
	_, err := um.GetUserById(id)
	if err != nil {
		_ = um.NewUser(id)
	}

	r := manager.redis.Get()
	defer manager.redis.Put(r)
	r.Hset("Key", key, []byte(id))
	return key
}

func (um *UserManager) run() {
	for {
		select {
		case c := <-um.register:
			c.user.conns[c] = true
		case c := <-um.unregister:
			if c.user != nil {
				delete(c.user.conns, c)
			}
		case m := <-um.broadcast:
			cnt := 0
			for c := range m.user.conns {
				if c != m.conn {
					go c.Send(m.packet)
					cnt++
				}
			}
			logger.Printf("[%s] broadcast to %d clients\n", m.user.Id, cnt)
		}
	}
}

type User struct {
	sync.RWMutex
	Id    string
	conns map[*Connection]bool
}

func (u *User) ChannelKey() string {
	return fmt.Sprintf("Channels:%s", u.Id)
}

func (u *User) ServerKey() string {
	return fmt.Sprintf("Servers:%s", u.Id)
}

func (u *User) GetLogId() int64 {
	r := manager.redis.Get()
	defer manager.redis.Put(r)
	id, _ := r.Incr(fmt.Sprintf("logid:%s", u.Id))
	return id
}

func (u *User) GetServerId() int {
	r := manager.redis.Get()
	defer manager.redis.Put(r)
	id, _ := r.Incr(fmt.Sprintf("serverid:%s", u.Id))
	return int(id)
}

func (u *User) GetServers() (servers []*common.IRCServer) {
	r := manager.redis.Get()
	defer manager.redis.Put(r)

	value, err := r.Hgetall(u.ServerKey())
	if err != nil {
		logger.Println("GetServers Error : ", err)
		return
	}
	result := common.Convert(value)

	for _, v := range result {
		var info common.IRCServer
		json.Unmarshal([]byte(v), &info)
		servers = append(servers, &info)
	}
	return
}

func (u *User) GetChannels() (channels []*common.IRCChannel) {
	r := manager.redis.Get()
	defer manager.redis.Put(r)

	value, err := r.Hgetall(u.ChannelKey())
	if err != nil {
		logger.Println("GetChannels Error : ", err)
		return
	}

	result := common.Convert(value)
	for k, v := range result {
		value, err := r.Hgetall(k)
		if err != nil {
			logger.Println("GetChannels Error : ", err)
			return
		}
		m := common.Convert(value)
		var channel common.IRCChannel
		common.Import(m, &channel)
		channel.Server_id, _ = strconv.Atoi(v)
		channels = append(channels, &channel)
	}
	return
}

func (u *User) AddServer(server *common.IRCServer) (*common.IRCServer, error) {
	server.Id = u.GetServerId()
	server.Active = false

	r := manager.redis.Get()
	defer manager.redis.Put(r)

	data, _ := json.Marshal(server)
	err := r.Hset(u.ServerKey(), strconv.Itoa(server.Id), data)
	if err != nil {
		logger.Println("AddServer Error : ", err)
		return nil, err
	}

	msg := &common.ZmqMsg{
		Cmd:      "ADD_SERVER",
		UserId:   u.Id,
		ServerId: server.Id,
		Params: map[string]interface{}{
			"serverinfo": server.Server,
			"userinfo":   server.User,
		},
	}

	manager.zmq.Send <- msg
	return server, nil
}

func (u *User) AddChannelMsg(serverid int, channel string) {
	msg := &common.ZmqMsg{
		Cmd:      "ADD_CHANNEL",
		UserId:   u.Id,
		ServerId: serverid,
		Params: map[string]interface{}{
			"channel": channel,
		},
	}

	manager.zmq.Send <- msg
}

func (u *User) GetPastLogs(last_log_id, numLogs, serverid int, channel string) ([]*common.IRCLog, error) {
	r := manager.redis.Get()
	defer manager.redis.Put(r)

	//	key := fmt.Sprintf("log:%s:%s:%s", u.Id, serverid, channel)

	return nil, nil
}

func (u *User) GetInitLogs(numLogs int) ([]*common.IRCLog, error) {
	r := manager.redis.Get()
	defer manager.redis.Put(r)

	value, err := r.Hgetall(u.ChannelKey())
	if err != nil {
		logger.Println("GetInitLogs Error : ", err)
		return nil, err
	}

	result := common.Convert(value)

	var logs []*common.IRCLog
	for k, v := range result {
		channel := strings.Split(k, "@")[0]
		key := fmt.Sprintf("log:%s:%s:%s", u.Id, v, channel)
		result, err := r.Lrange(key, 0, int64(numLogs))
		if err != nil {
			logger.Println("GetInitLogs Error : ", err)
			return nil, err
		}
		for _, b := range result {
			var log common.IRCLog
			json.Unmarshal(b, &log)
			logs = append(logs, &log)
		}
	}

	return logs, nil
}

func (u *User) Send(packet *Packet, conn *Connection) {
	manager.user.broadcast <- &UserMessage{user: u, packet: packet, conn: conn}
}

func (u *User) SendChatMsg(serverId int, target, message string) {
	msg := &common.ZmqMsg{
		Cmd:      "SEND_CHAT",
		UserId:   u.Id,
		ServerId: serverId,
		Params:   map[string]interface{}{"target": target, "message": message},
	}
	manager.zmq.Send <- msg
}

type UserMessage struct {
	user   *User
	packet *Packet
	conn   *Connection
}

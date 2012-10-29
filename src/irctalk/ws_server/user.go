package main

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"io"
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
	sync.RWMutex
	users      map[string]*User
	caches     map[string]string
	register   chan *Connection
	unregister chan *Connection
	broadcast  chan *UserMessage
}

func (um *UserManager) GetUserByKey(key string) (*User, error) {
	r := manager.redis.Get()
	defer manager.redis.Put(r)

	id, err := r.Hget("key", key)
	if err != nil || id == nil {
		return nil, &CacheNotFound{key: key}
	}
	return um.GetUserById(string(id))
}

func (um *UserManager) GetConnectedUser(id string) (*User, error) {
	um.RLock()
	defer um.RUnlock()

	user, ok := um.users[id]
	if !ok {
		return nil, &UserNotFound{id: id}
	}
	return user, nil
}

func (um *UserManager) GetUserById(id string) (*User, error) {
	um.RLock()
	user, ok := um.users[id]
	um.RUnlock()
	if !ok {
		um.Lock()
		defer um.Unlock()
		user = um.LoadUser(id)
		if user == nil {
			logger.Println("User Not Found")
			return nil, &UserNotFound{id: id}
		}
		if _, ok := um.users[id]; !ok {
			um.users[id] = user
		}
	}
	return user, nil
}

func (um *UserManager) LoadUser(id string) *User {
	r := manager.redis.Get()
	defer manager.redis.Put(r)

	user := &User{
		Id:      id,
		servers: make(map[int]*common.IRCServer),
		conns:   make(map[*Connection]bool),
	}

	value, err := r.Hgetall(user.ServerKey())
	if err != nil {
		logger.Println("LoadUser Error : ", err)
		return nil
	}
	result := common.Convert(value)

	for k, v := range result {
		var info common.IRCServer
		json.Unmarshal([]byte(v), &info)
		serverid, _ := strconv.Atoi(k)
		user.servers[serverid] = &info
	}

	return user
}

func (um *UserManager) RegisterUser(id string) string {
	// make key
	seed, _ := time.Now().GobEncode()
	h := hmac.New(sha1.New, seed)
	io.WriteString(h, id)
	key := fmt.Sprintf("%0x", h.Sum(nil))
	um.GetUserById(id)

	r := manager.redis.Get()
	defer manager.redis.Put(r)
	r.Hset("key", key, []byte(id))
	return key
}

func (um *UserManager) run() {
	for {
		select {
		case c := <-um.register:
			um.Lock()
			c.user.conns[c] = true
			um.Unlock()
		case c := <-um.unregister:
			if c.user != nil {
				delete(c.user.conns, c)
				if len(c.user.conns) == 0 {
					um.Lock()
					delete(um.users, c.user.Id)
					um.Unlock()
				}
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
	Id      string
	servers map[int]*common.IRCServer
	conns   map[*Connection]bool
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

	u.RLock()
	defer u.RUnlock()
	servers = make([]*common.IRCServer, 0)
	for _, v := range u.servers {
		servers = append(servers, v)
	}
	return
}

func (u *User) GetChannels() (channels []*common.IRCChannel) {
	r := manager.redis.Get()
	defer manager.redis.Put(r)

	channels = make([]*common.IRCChannel, 0)
	value, err := r.Hgetall(u.ChannelKey())
	if err != nil {
		logger.Println("GetChannels Error : ", err)
		return
	}

	result := common.Convert(value)
	for k, _ := range result {
		value, err := r.Get(k)
		if err != nil {
			logger.Println("GetChannels Error : ", err)
			return
		}
		var channel common.IRCChannel
		json.Unmarshal(value, &channel)
		channels = append(channels, &channel)
	}
	return
}

func (u *User) AddServer(server *common.IRCServer) (*common.IRCServer, error) {
	server.Id = u.GetServerId()
	server.Active = false

	u.Lock()
	defer u.Unlock()

	r := manager.redis.Get()
	defer manager.redis.Put(r)

	data, _ := json.Marshal(server)
	err := r.Hset(u.ServerKey(), strconv.Itoa(server.Id), data)
	if err != nil {
		logger.Println("AddServer Error : ", err)
		return nil, err
	}

	u.servers[server.Id] = server

	manager.zmq.Send <- common.MakeZmqMsg(u.Id, server.Id, common.ZmqAddServer{ServerInfo: server})

	return server, nil
}

func (u *User) AddChannelMsg(serverid int, channel string) {
	manager.zmq.Send <- common.MakeZmqMsg(u.Id, serverid, common.ZmqAddChannel{Channel: &common.IRCChannel{Name: channel}})
}

func (u *User) GetPastLogs(last_log_id, numLogs, serverid int, channel string) ([]*common.IRCLog, error) {
	r := manager.redis.Get()
	defer manager.redis.Put(r)

	//key := fmt.Sprintf("log:%s:%d:%s", u.Id, serverid, channel)

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

	logs := make([]*common.IRCLog, 0)
	for k, v := range result {
		channel := strings.Split(k, ":")[3]
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

func (u *User) SendChatMsg(serverid int, target, message string) {
	manager.zmq.Send <- common.MakeZmqMsg(u.Id, serverid, common.ZmqSendChat{Target: target, Message: message})
}

func (u *User) ChangeServerActive(serverid int, active bool) {
	u.Lock()
	defer u.Unlock()
	server, ok := u.servers[serverid]
	if !ok {
		logger.Println("Server not found :", serverid)
		return
	}

	logger.Println("Current State:", server.Active, active)
	if server.Active == active {
	}

	server.Active = active

	packet := &Packet{
		Cmd:     "serverActive",
		RawData: map[string]interface{}{"server_id": serverid, "active": active},
	}
	u.Send(packet, nil)
}

type UserMessage struct {
	user   *User
	packet *Packet
	conn   *Connection
}

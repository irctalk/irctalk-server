package main

import (
	"crypto/hmac"
	"crypto/sha1"
	"fmt"
	"io"
	"irctalk/common"
	"redigo/redis"
	"sync"
	"time"
	"log"
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
	r := common.DefaultRedisPool().Get()
	defer r.Close()

	id, err := redis.String(r.Do("HGET", "key", key))
	if err != nil {
		return nil, &CacheNotFound{key: key}
	}
	return um.GetUserById(id)
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
			log.Println("User Not Found")
			return nil, &UserNotFound{id: id}
		}
		if _, ok := um.users[id]; !ok {
			um.users[id] = user
		}
	}
	return user, nil
}

func (um *UserManager) LoadUser(id string) *User {
	user := &User{
		Id:          id,
		conns:       make(map[*Connection]bool),
		serverIdSeq: &common.RedisNumber{Key: fmt.Sprintf("serverid:%s", id)},
		logIdSeq:    &common.RedisNumber{Key: fmt.Sprintf("logid:%s", id)},
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

	r := common.DefaultRedisPool().Get()
	defer r.Close()
	_, err := r.Do("HSET", "key", key, id)
	if err != nil {
		log.Println("RegisterUser: ", err)
	}
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
			log.Printf("[%s] broadcast to %d clients\n", m.user.Id, cnt)
		}
	}
}

type User struct {
	sync.RWMutex
	Id          string
	conns       map[*Connection]bool
	serverIdSeq *common.RedisNumber
	logIdSeq    *common.RedisNumber
}

func (u *User) ServerListKey() string {
	return fmt.Sprintf("servers:%s", u.Id)
}
func (u *User) ChannelListKey() string {
	return fmt.Sprintf("channels:%s", u.Id)
}

func (u *User) GetServers() (servers []*common.IRCServer) {
	err := common.RedisSliceLoad(u.ServerListKey(), &servers)
	if err != nil {
		log.Println("GetServers: ", err)
	}
	return
}

func (u *User) GetChannels() (channels []*common.IRCChannel) {
	err := common.RedisSliceLoad(u.ChannelListKey(), &channels)
	if err != nil {
		log.Println("GetChannels: ", err)
		return nil
	}
	return
}

func (u *User) AddServer(server *common.IRCServer) (*common.IRCServer, error) {
	server.Id = int(u.serverIdSeq.Incr())
	server.UserId = u.Id
	server.Active = false

	servers := []*common.IRCServer{server}

	err := common.RedisSliceSave(u.ServerListKey(), &servers)
	if err != nil {
		return nil, err
	}

	manager.zmq.Send <- common.MakeZmqMsg(u.Id, server.Id, common.ZmqAddServer{ServerInfo: server})

	return server, nil
}

func (u *User) AddChannelMsg(serverid int, channel string) {
	manager.zmq.Send <- common.MakeZmqMsg(u.Id, serverid, common.ZmqAddChannel{Channel: &common.IRCChannel{Name: channel}})
}

func (u *User) GetPastLogs(lastLogId int64, numLogs, serverId int, channel string) ([]*common.IRCLog, error) {
	return common.GetPastLogs(u.Id, serverId, channel, lastLogId, numLogs)
}

func (u *User) GetInitLogs(lastLogId int64, numLogs int) ([]*common.IRCLog, error) {
	channels := u.GetChannels()
	logs := make([]*common.IRCLog, 0)
	for _, channel := range channels {
		_logs, err := common.GetLastLogs(u.Id, channel.ServerId, channel.Name, lastLogId, numLogs)
		if err != nil {
			return nil, err
		}
		logs = append(logs, _logs...)
	}
	return logs, nil
}

func (u *User) GetServer(serverId int) (*common.IRCServer, error) {
	server := &common.IRCServer{UserId: u.Id, Id: serverId}
	err := common.RedisLoad(server)
	if err != nil {
		return nil, err
	}
	return server, nil
}

func (u *User) Send(packet *Packet, conn *Connection) {
	manager.user.broadcast <- &UserMessage{user: u, packet: packet, conn: conn}
}

func (u *User) SendChatMsg(serverid int, target, message string) {
	manager.zmq.Send <- common.MakeZmqMsg(u.Id, serverid, common.ZmqSendChat{Target: target, Message: message})
}

func (u *User) ChangeServerActive(serverId int, active bool) {
	packet := MakePacket(&SendServerActive{ServerId:serverId, Active:active})
	u.Send(packet, nil)
}

type UserMessage struct {
	user   *User
	packet *Packet
	conn   *Connection
}

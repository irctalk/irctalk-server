package main

import (
	"fmt"
	"irctalk/common"
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
	id, ok := um.caches[key]
	if !ok {
		return nil, &CacheNotFound{key: key}
	}
	return um.GetUserById(id)
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
		Id:      id,
		um:      um,
		conns:   make(map[*Connection]bool),
		noConns: make(chan bool),
		log_id:  make(chan int64),
	}
	return um.users[id]
}

func (um *UserManager) RegisterUser(id string) string {
	// make key
	key := id
	_, err := um.GetUserById(id)
	if err != nil {
		_ = um.NewUser(id)
	}
	um.caches[key] = id
	return key
}

func (um *UserManager) run() {
	for {
		select {
		case c := <-um.register:
			c.user.conns[c] = true
			if len(c.user.conns) == 1 {
				go c.user.PushLogTest()
			}
		case c := <-um.unregister:
			if c.user != nil {
				delete(c.user.conns, c)
				if len(c.user.conns) == 0 {
					c.user.noConns <- true
				}
			}
		case m := <-um.broadcast:
			for c := range m.user.conns {
				go c.Send(m.packet)
			}
		}
	}
}

type User struct {
	sync.RWMutex
	Id      string
	um      *UserManager
	conns   map[*Connection]bool
	noConns chan bool

	// for test
	log_id chan int64
}

func (u *User) GetServers() []*common.IRCServerInfo {
	return []*common.IRCServerInfo{common.TestServerInfo}
}

func (u *User) GetLogs() []*common.IRCLog {
	return []*common.IRCLog{common.TestLog}
}

func (u *User) Send(packet *Packet) {
	manager.user.broadcast <- &UserMessage{user: u, packet: packet}
}

func (u *User) PushLogTest() {
	tick := time.Tick(1 * time.Second)
	go func() {
		log_id := int64(0)
		for {
			select {
			case u.log_id <- log_id:
				log_id++
			}
		}
	}()
	for {
		select {
		case t := <-tick:
			i := <-u.log_id
			irclog := &common.IRCLog{
				Log_id:    i,
				Server_id: 0,
				Timestamp: common.UnixMilli(t),
				Channel:   "#test",
				From:      "irctalk",
				Message:   fmt.Sprintf("test push message #%d", i),
			}
			packet := &Packet{Cmd: "pushLogs", RawData: make(map[string]interface{})}
			packet.RawData["logs"] = []*common.IRCLog{irclog}
			logger.Printf("%+v\n", *packet)
			u.Send(packet)
		case <-u.noConns:
			logger.Println("There is No Connection.")
			return
		}
	}
}

type UserMessage struct {
	user   *User
	packet *Packet
}

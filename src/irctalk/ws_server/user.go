package main

import (
	"crypto/hmac"
	"crypto/sha1"
	"fmt"
	"io"
	"irctalk/common"
	"log"
	"redigo/redis"
	"sync"
	"sync/atomic"
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

func (um *UserManager) SendPushMessage(userId string, packet *Packet) error {
	_tokens := make([]string, 0)
	tokenListKey := fmt.Sprintf("tokens:%s", userId)

	err := common.RedisSliceLoad(tokenListKey, &_tokens)
	if err != nil {
		return err
	}

	tokens := make(map[string]bool)
	for _, token := range _tokens {
		tokens[token] = true
	}

	user, _ := um.GetConnectedUser(userId)
	if user != nil {
		// websocket connection으로 푸시 전송 시도
		ignoreTokens := user.SendPushMessage(packet)
		for token, _ := range ignoreTokens {
			delete(tokens, token)
		}
	}

	if len(tokens) != 0 {
		// 푸시 Agent로 푸시 전송 시도
		pushMessage := &PushMessage{
			UserId:     userId,
			PushTokens: make([]string, len(tokens)),
			Payload:    packet,
		}
		i := 0
		for token, _ := range tokens {
			pushMessage.PushTokens[i] = token
			i++
		}
		manager.push.Send <- pushMessage
	}
	return nil
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
		Id:           id,
		conns:        make(map[*Connection]string),
		serverIdSeq:  &common.RedisNumber{Key: fmt.Sprintf("serverid:%s", id)},
		logIdSeq:     &common.RedisNumber{Key: fmt.Sprintf("logid:%s", id)},
		waitingTimer: make(map[int]*time.Timer),
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
			if c.isClosed {
				log.Println("Ignore Closed Connection")
			} else {
				c.user.Lock()
				c.user.conns[c] = c.pushToken
				c.user.Unlock()
			}
		case c := <-um.unregister:
			if c.user != nil {
				c.user.Lock()
				delete(c.user.conns, c)
				c.user.Unlock()
				if len(c.user.conns) == 0 {
					um.Lock()
					delete(um.users, c.user.Id)
					um.Unlock()
				}
			}
		case m := <-um.broadcast:
			cnt := 0
			m.user.RLock()
			for c := range m.user.conns {
				if c != m.conn {
					go c.Send(m.packet)
					cnt++
				}
			}
			m.user.RUnlock()
			log.Printf("[%s] broadcast to %d clients\n", m.user.Id, cnt)
		}
	}
}

type User struct {
	sync.RWMutex
	Id           string
	conns        map[*Connection]string
	serverIdSeq  *common.RedisNumber
	logIdSeq     *common.RedisNumber
	msgIdSeq     int32
	waitingTimer map[int]*time.Timer
}

func (u *User) ServerListKey() string {
	return fmt.Sprintf("servers:%s", u.Id)
}
func (u *User) ChannelListKey() string {
	return fmt.Sprintf("channels:%s", u.Id)
}
func (u *User) PushTokenListKey() string {
	return fmt.Sprintf("tokens:%s", u.Id)
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
	_channel := &common.IRCChannel{Name: channel}
	manager.zmq.Send <- common.MakeZmqMsg(u.Id, serverid, common.ZmqAddChannel{Channel: _channel})
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
	packet := MakePacket(&SendServerActive{ServerId: serverId, Active: active})
	u.Send(packet, nil)
}

func (u *User) GetNotification(pushType, pushToken string) (bool, error) {
	r := common.DefaultRedisPool().Get()
	defer r.Close()

	token := fmt.Sprintf("%s:%s", pushType, pushToken)
	alert, err := redis.Int(r.Do("SISMEMBER", u.PushTokenListKey(), token))

	return alert == 1, err
}

func (u *User) SetNotification(pushType, pushToken string, isAlert bool) (err error) {
	tokens := []string{fmt.Sprintf("%s:%s", pushType, pushToken)}
	if isAlert {
		err = common.RedisSliceSave(u.PushTokenListKey(), &tokens)
	} else {
		err = common.RedisSliceRemove(u.PushTokenListKey(), &tokens)
	}
	return
}

func (u *User) JoinPartChannel(serverId int, channel string, join bool) {
	manager.zmq.Send <- common.MakeZmqMsg(u.Id, serverId, common.ZmqJoinPartChannel{Channel: channel, Join: join})
}

func (u *User) DelChannel(serverId int, channel string) {
	manager.zmq.Send <- common.MakeZmqMsg(u.Id, serverId, common.ZmqDelChannel{Channel: channel})
}

func (u *User) AckPushMessage(msgId int) {
	u.Lock()
	defer u.Unlock()

	t, ok := u.waitingTimer[msgId]
	if !ok {
		log.Println("AckPushMessage received too late: ", msgId)
		return
	}
	delete(u.waitingTimer, msgId)
	t.Stop()
}

func (u *User) SendPushMessage(packet *Packet) map[string]int {
	u.Lock()
	defer u.Unlock()

	tokens := make(map[string]int)
	for c, token := range u.conns {
		msgId, ok := tokens[token]
		if !ok {
			msgId = int(atomic.AddInt32(&u.msgIdSeq, 1))
		}
		p := &Packet{Cmd: packet.Cmd, MsgId: msgId, body: packet.body}
		go c.Send(p)

		// 토큰이 있는 커넥션들에 대해서 처리
		// 같은 토큰에 대해선 타이머에 등록하지 않는다.
		log.Printf("Token: [%s]", token)
		if token != "" && !ok {
			_token := token
			u.waitingTimer[p.MsgId] = time.AfterFunc(time.Duration(common.Config.PushResponseTimeout)*time.Second, func() {
				u.Lock()
				defer u.Unlock()
				// timer function에 진입 했지만 AckPushMessage에서 먼저 lock을 잡은상태에서 stop을 하고 제거를 했을수도 있음
				if _, ok := u.waitingTimer[p.MsgId]; !ok {
					return
				}
				delete(u.waitingTimer, p.MsgId)
				log.Printf("Send PushMessage via websocket connection failed. Token:%s [%s](%d) %s", _token, p.Cmd, p.MsgId, string(p.RawData))
				// send to agent
				manager.push.Send <- &PushMessage{UserId: u.Id, PushTokens: []string{_token}, Payload: p}
			})
			tokens[token] = p.MsgId
		}
	}

	return tokens
}

type UserMessage struct {
	user   *User
	packet *Packet
	conn   *Connection
}

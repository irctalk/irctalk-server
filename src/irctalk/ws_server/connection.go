package main

import (
	"code.google.com/p/go.net/websocket"
	"encoding/json"
	"irctalk/common"
	"log"
	"time"
)

type Connection struct {
	ws       *websocket.Conn
	send     chan *Packet
	user     *User
	handler  PacketHandler
	stoprecv chan bool
	isClosed bool
}

func AuthUser(f func(*Connection, *Packet)) func(*Connection, *Packet) {
	return func(c *Connection, packet *Packet) {
		if c.user != nil {
			f(c, packet)
		} else {
			resp := packet.MakeResponse()
			defer c.Send(resp)
			resp.Status = -401
			resp.Msg = "Authorization Required"
		}
	}
}

func MakeDefaultPacketHandler() *PacketMux {
	h := NewPacketMux()
	h.HandleFunc("register", func(c *Connection, packet *Packet) {
		resp := packet.MakeResponse()
		defer c.Send(resp)
		reqBody := packet.body.(*ReqRegister)
		resBody := resp.body.(*ResRegister)

		g := NewGoogleOauth(reqBody.AccessToken)
		userinfo, err := g.GetUserInfo()
		if err != nil {
			log.Println("GetUserInfo Error:", err)
			resp.Status = -500
			resp.Msg = err.Error()
			return
		}
		log.Printf("%+v\n", userinfo)
		id, ok := userinfo["id"].(string)
		if !ok {
			log.Println("oauth Error!")
			resp.Status = -500
			resp.Msg = "Invalid Access Token"
			return
		}
		resBody.AuthKey = manager.user.RegisterUser(id)
	})

	h.HandleFunc("login", func(c *Connection, packet *Packet) {
		resp := packet.MakeResponse()
		defer c.Send(resp)
		reqBody := packet.body.(*ReqLogin)
		resBody := resp.body.(*ResLogin)

		user, err := manager.user.GetUserByKey(reqBody.AuthKey)
		if err != nil {
			log.Println("[Login] GetUserInfo Error:", err)
			resp.Status = -401
			resp.Msg = err.Error()
			return
		}
		c.user = user

		// add connection to user
		manager.user.register <- c

		if reqBody.PushType != "" {
			resBody.Alert, err = c.user.GetNotification(reqBody.PushType, reqBody.PushToken)
			if err != nil {
				log.Println("[Login] GetPushAlertStatus Error:", err)
				resp.Status = -500
				resp.Msg = err.Error()
				return
			}
		}

		log.Printf("%+v\n", user)
	})

	h.HandleFunc("getServers", AuthUser(func(c *Connection, packet *Packet) {
		resp := packet.MakeResponse()
		defer c.Send(resp)
		resBody := resp.body.(*ResGetServers)

		resBody.Servers = c.user.GetServers()
		resBody.Channels = c.user.GetChannels()

		log.Printf("%+v %+v\n", resp, resBody)
	}))

	h.HandleFunc("getInitLogs", AuthUser(func(c *Connection, packet *Packet) {
		resp := packet.MakeResponse()
		defer c.Send(resp)
		reqBody := packet.body.(*ReqGetInitLogs)
		resBody := resp.body.(*ResGetInitLogs)

		lastLogId := int64(-1)
		if reqBody.LastLogId != 0 {
			lastLogId = reqBody.LastLogId
		}

		var err error
		resBody.Logs, err = c.user.GetInitLogs(lastLogId, reqBody.LogCount)
		if err != nil {
			log.Printf("getInitLog Error :", err)
			resp.Status = -500
			resp.Msg = err.Error()
			return
		}
		log.Printf("%+v %+v\n", resp, resBody)
	}))

	h.HandleFunc("getPastLogs", AuthUser(func(c *Connection, packet *Packet) {
		resp := packet.MakeResponse()
		defer c.Send(resp)
		reqBody := packet.body.(*ReqGetPastLogs)
		resBody := resp.body.(*ResGetPastLogs)

		var err error
		resBody.Logs, err = c.user.GetPastLogs(reqBody.LastLogId, reqBody.LogCount, reqBody.ServerId, reqBody.Channel)
		if err != nil {
			log.Printf("getPastLogs Error :", err)
			resp.Status = -500
			resp.Msg = err.Error()
			return
		}
		log.Printf("%+v %+v\n", resp, resBody)
	}))

	h.HandleFunc("pushLog", AuthUser(func(c *Connection, packet *Packet) {
		log.Printf("%+v\n", packet)
	}))

	h.HandleFunc("sendLog", AuthUser(func(c *Connection, packet *Packet) {
		resp := packet.MakeResponse()
		defer c.Send(resp)
		reqBody := packet.body.(*ReqSendLog)
		resBody := resp.body.(*ResSendLog)

		server, err := c.user.GetServer(reqBody.ServerId)
		if err != nil || !server.Active {
			log.Println("SendLog failed. server is not connected")
			resp.Status = -500
			resp.Msg = "Server is not connected"
			return
		}

		irclog := &common.IRCLog{
			UserId:    c.user.Id,
			LogId:     c.user.logIdSeq.Incr(),
			ServerId:  reqBody.ServerId,
			Timestamp: common.UnixMilli(time.Now()),
			Channel:   reqBody.Channel,
			From:      server.User.Nickname,
			Message:   reqBody.Message,
		}
		c.user.SendChatMsg(reqBody.ServerId, irclog.Channel, irclog.Message)
		resBody.Log = irclog
		c.user.Send(MakePacket(&SendPushLog{Log:irclog}), c)
	}))

	h.HandleFunc("addServer", AuthUser(func(c *Connection, packet *Packet) {
		resp := packet.MakeResponse()
		defer c.Send(resp)
		reqBody := packet.body.(*ReqAddServer)
		resBody := resp.body.(*ResAddServer)

		var err error
		resBody.Server, err = c.user.AddServer(reqBody.Server)
		if err != nil {
			log.Println("AddServer Error :", err)
			resp.Status = -500
			resp.Msg = err.Error()
			return
		}
		c.user.Send(resp, c)
	}))

	h.HandleFunc("addChannel", AuthUser(func(c *Connection, packet *Packet) {
		reqBody := packet.body.(*ReqAddChannel)
		c.user.AddChannelMsg(reqBody.ServerId, reqBody.Channel)
	}))

	h.HandleFunc("setNotification", AuthUser(func(c *Connection, packet *Packet) {
		resp := packet.MakeResponse()
		defer c.Send(resp)
		reqBody := packet.body.(*ReqSetNotification)

		if err := c.user.SetNotification(reqBody.PushType, reqBody.PushToken, reqBody.Alert); err != nil {
			log.Println("SetNotification Error : ", err)
			resp.Status = -500
			resp.Msg = err.Error()
			return
		}
	}))

	return h
}

type ConnectionManager struct {
	connections map[*Connection]bool
	register    chan *Connection
	unregister  chan *Connection
	h           *PacketMux
}

func (cm *ConnectionManager) run() {
	cm.h = MakeDefaultPacketHandler()
	for i := 0; i < cm.h.n_worker; i++ {
		go cm.h.Worker()
	}
	for {
		select {
		case c := <-cm.register:
			cm.connections[c] = true
		case c := <-cm.unregister:
			delete(cm.connections, c)
			// delete connection from user
			manager.user.unregister <- c
			close(c.send)
			c.isClosed = true
		}
	}
}

func (c *Connection) reader() {
	recv := make(chan *Packet)
	stop := make(chan bool, 1)
	c.stoprecv = make(chan bool, 1)
STOP:
	for {
		go func() {
			var packet *Packet
			err := websocket.JSON.Receive(c.ws, &packet)
			if err != nil {
				log.Println("Read error: ", err)
				stop <- true
				return
			}
			recv <- packet
		}()
		select {
		case packet := <-recv:
			log.Printf("-> [%s](%d): %s", packet.Cmd, packet.Status, string(packet.RawData))
			c.handler.Handle(c, packet)
		case <-c.stoprecv:
			break STOP
		case <-stop:
			break STOP
		}
	}
	c.ws.Close()
	log.Println("closed")
}

func (c *Connection) writer() {
	for packet := range c.send {
		log.Println("try to write packet")
		c.ws.SetWriteDeadline(time.Now().Add(10 * time.Second))
		err := websocket.JSON.Send(c.ws, packet)
		if err != nil {
			log.Println("Write error: ", err)
			break
		}
		log.Println("success to write packet")
	}
	c.ws.Close()
	c.stoprecv <- true
	log.Println("Write Closed")
}

func (c *Connection) Send(packet *Packet) {
	defer func() {
		if x := recover(); x != nil {
			log.Println("Connection Closed. This Packet will be dropped.", x)
		}
	}()

	var err error
	packet.RawData, err = json.Marshal(packet.body)
	if err != nil {
		log.Println("Json Marshal Error:", packet.body, err)
		return
	}
	log.Printf("<- [%s](%d): %s", packet.Cmd, packet.Status, string(packet.RawData))
	select {
	case c.send <- packet:
	case <-time.After(2 * time.Second):
		log.Printf("Send Buffer is Full. Packet will be dropped.")
	}
}

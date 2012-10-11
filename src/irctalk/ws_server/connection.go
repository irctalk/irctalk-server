package main

import (
	"code.google.com/p/go.net/websocket"
	"irctalk/common"
	"time"
)

type Connection struct {
	ws          *websocket.Conn
	send        chan *Packet
	user        *User
	handler     PacketHandler
	last_log_id int64
	stoprecv    chan bool
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
		token := packet.RawData["access_token"].(string)
		g := NewGoogleOauth(token)
		userinfo, err := g.GetUserInfo()
		if err != nil {
			logger.Println("GetUserInfo Error:", err)
			resp.Status = -500
			resp.Msg = err.Error()
			return
		}
		logger.Printf("%+v\n", userinfo)
		id, ok := userinfo["id"].(string)
		if !ok {
			logger.Println("oauth Error!")
			resp.Status = -500
			resp.Msg = "Invalid Access Token"
			return
		}
		resp.RawData["auth_key"] = manager.user.RegisterUser(id)
	})

	h.HandleFunc("login", func(c *Connection, packet *Packet) {
		resp := packet.MakeResponse()
		defer c.Send(resp)
		key := packet.RawData["auth_key"].(string)
		user, err := manager.user.GetUserByKey(key)
		if err != nil {
			logger.Println("[Login] GetUserInfo Error:", err)
			resp.Status = -401
			resp.Msg = err.Error()
			return
		}
		c.user = user

		// add connection to user
		manager.user.register <- c

		logger.Printf("%+v\n", user)
	})

	h.HandleFunc("getServers", AuthUser(func(c *Connection, packet *Packet) {
		resp := packet.MakeResponse()
		defer c.Send(resp)
		resp.RawData["servers"] = c.user.GetServers()
		logger.Printf("%+v\n", resp)
	}))

	h.HandleFunc("getLogs", AuthUser(func(c *Connection, packet *Packet) {
		resp := packet.MakeResponse()
		defer c.Send(resp)
		resp.RawData["logs"] = c.user.GetLogs()
		logger.Printf("%+v\n", resp)
	}))

	h.HandleFunc("pushLog", AuthUser(func(c *Connection, packet *Packet) {
		logger.Printf("%+v\n", packet)
	}))

	h.HandleFunc("sendLog", AuthUser(func(c *Connection, packet *Packet) {
		resp := packet.MakeResponse()
		defer c.Send(resp)
		irclog := &common.IRCLog{
			Log_id:    <-c.user.log_id,
			Server_id: int(packet.RawData["server_id"].(float64)),
			Timestamp: common.UnixMilli(time.Now()),
			Channel:   packet.RawData["channel"].(string),
			From:      "irctalk",
			Message:   packet.RawData["message"].(string),
		}

		resp.RawData["log"] = irclog
		push := &Packet{Cmd: "pushLog", RawData: map[string]interface{}{"log": irclog}}
		c.user.Send(push, c)
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
				logger.Println("Read error: ", err)
				stop <- true
				return
			}
			recv <- packet
		}()
		select {
		case packet := <-recv:
			logger.Printf("%+v\n", packet)
			c.handler.Handle(c, packet)
		case <-c.stoprecv:
			break STOP
		case <-stop:
			break STOP
		}
	}
	c.ws.Close()
	logger.Println("closed")
}

func (c *Connection) writer() {
	for packet := range c.send {
		logger.Println("try to write packet")
		c.ws.SetWriteDeadline(time.Now().Add(10*time.Second))
		err := websocket.JSON.Send(c.ws, packet)
		if err != nil {
			logger.Println("Write error: ", err)
			break
		}
		logger.Println("success to write packet")
	}
	c.ws.Close()
	c.stoprecv <- true
	logger.Println("Write Closed")
}

func (c *Connection) Send(packet *Packet) {
	defer func() {
		if x := recover(); x != nil {
			logger.Printf("Connection Closed. This Packet will be dropped.")
		}
	}()
	select {
	case c.send <- packet:
	case <-time.After(2 * time.Second):
		logger.Printf("Send Buffer is Full. Packet will be dropped.")
	}
}

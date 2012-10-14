package main

import (
	"irctalk/common"
	irc "github.com/fluffle/goirc/client"
	"log"
	"hash/crc32"
	"fmt"
	"crypto/tls"
)

type IRCManager struct {
	clients map[string]*IRCClient
	register chan *IRCClient
}

func (im *IRCManager) run() {
	for {
		select {
		case c := <- im.register:
			im.clients[c.Id] = c
			c.Connect()
		}
	}
}

func (im *IRCManager) GetClient(msg *common.ZmqMsg) *IRCClient {
	c, ok := im.clients[msg.GetClientId()]
	if !ok {
		return nil
	}
	return c
}

type IRCClient struct {
	Id string
	UserId string
	ServerId int
	serverInfo *common.IRCServerInfo
	userInfo *common.IRCUser
	conn *irc.Conn
	channels map[string]bool
}

func NewClient(info *common.ZmqMsg) *IRCClient{
	var serverInfo common.IRCServerInfo
	var userInfo common.IRCUser
	log.Printf("%T\n", info.Params["serverinfo"])
	common.Import(info.Params["serverinfo"], &serverInfo)
	common.Import(info.Params["userinfo"], &userInfo)
	ident := fmt.Sprintf("%0X", crc32.ChecksumIEEE([]byte(info.UserId)))

	client := &IRCClient {
		Id: info.GetClientId(),
		UserId: info.UserId,
		ServerId: 0, // TODO: manage serverid
		serverInfo: &serverInfo,
		userInfo: &userInfo,
		conn: irc.SimpleClient(userInfo.Nickname, ident, userInfo.Realname),
		channels: make(map[string]bool),
	}
	client.conn.SSL = serverInfo.SSL
	client.conn.SSLConfig = &tls.Config {
		InsecureSkipVerify: true,
	}

	client.conn.AddHandler("connected", func(conn *irc.Conn, line *irc.Line) {
		for channel, joined := range client.channels {
			if !joined {
				client.channels[channel] = true
				conn.Join(channel)
			}
		}
	})
	client.conn.AddHandler("PRIVMSG", func(conn *irc.Conn, line *irc.Line) {
		log.Printf("%+v\n", line)
		if len(line.Args) == 2 && line.Args[0][0] == '#' {
			msg := client.MakeZmqMsg("CHAT")
			msg.Params["from"] = line.Nick
			msg.Params["channel"] = line.Args[0]
			msg.Params["message"] = line.Args[1]
			msg.Params["time"] = common.UnixMilli(line.Time)
			zmqMgr.Send <- msg
		}
	})
	return client
}

func (c *IRCClient) MakeZmqMsg(cmd string) *common.ZmqMsg {
	return &common.ZmqMsg {
		Cmd: cmd,
		UserId: c.UserId,
		ServerId: 0,
		Params: make(map[string]interface{}),
	}
}

func (c* IRCClient) Connect() {
	addr := fmt.Sprintf("%s:%d", c.serverInfo.Host, c.serverInfo.Port)
	if err := c.conn.Connect(addr); err != nil {
		log.Println(err)
	}
}

func (c *IRCClient) SendLog(target, message string) {
	// write to redis for logging
	c.conn.Privmsg(target, message)
}

func (c *IRCClient) AddChannel(channel string) {
	c.channels[channel] = c.conn.Connected
	if c.conn.Connected {
		c.conn.Join(channel)
	}
}

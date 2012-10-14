package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	irc "github.com/fluffle/goirc/client"
	"hash/crc32"
	"irctalk/common"
	"log"
	"strconv"
	"strings"
	"time"
)

type IRCManager struct {
	clients  map[string]*IRCClient
	register chan *IRCClient
}

func (im *IRCManager) run() {
	for {
		select {
		case c := <-im.register:
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
	Id         string
	UserId     string
	ServerId   int
	serverInfo *common.IRCServer
	conn       *irc.Conn
	channels   map[string]*Channel
}

func NewClient(info *common.ZmqMsg) *IRCClient {
	var serverInfo common.IRCServer
	common.Import(info.Params["serverinfo"], &serverInfo)
	ident := fmt.Sprintf("%0X", crc32.ChecksumIEEE([]byte(info.UserId)))

	client := &IRCClient{
		Id:         info.GetClientId(),
		UserId:     info.UserId,
		ServerId:   serverInfo.Id,
		serverInfo: &serverInfo,
		conn:       irc.SimpleClient(serverInfo.User.Nickname, ident, serverInfo.User.Realname),
		channels:   make(map[string]*Channel),
	}
	client.conn.SSL = serverInfo.Server.SSL
	client.conn.SSLConfig = &tls.Config{
		InsecureSkipVerify: true,
	}

	client.conn.AddHandler("connected", func(conn *irc.Conn, line *irc.Line) {
		for _, channel := range client.channels {
			if !channel.joined {
				channel.joined = true
				conn.Join(channel.name)
			}
		}

		msg := client.MakeZmqMsg("SERVER_STATUS")
		msg.Params["active"] = true
		zmqMgr.Send <- msg

		client.serverInfo.Active = true
		client.WriteServerInfo()
	})

	client.conn.AddHandler("disconnected", func(conn *irc.Conn, line *irc.Line) {
		msg := client.MakeZmqMsg("SERVER_STATUS")
		msg.Params["active"] = false
		zmqMgr.Send <- msg

		client.serverInfo.Active = false
		client.WriteServerInfo()
	})

	client.conn.AddHandler("JOIN", func(conn *irc.Conn, line *irc.Line) {
		ircLog := client.WriteChatLog(line.Time, "", line.Args[0], fmt.Sprintf("%s has joined %s", line.Nick, line.Args[0]))

		msg := client.MakeZmqMsg("CHAT")
		msg.Params["log"] = ircLog
		zmqMgr.Send <- msg

		if line.Nick == conn.Me.Nick {
			// join channel by me
		} else {
		}
	})

	client.conn.AddHandler("332", func(conn *irc.Conn, line *irc.Line) {
		channel, ok := client.channels[line.Args[1]]
		if !ok {
			log.Println("Invalid Channel :", line.Args[1])
			return
		}
		channel.topic = line.Args[2]
	})

	client.conn.AddHandler("353", func(conn *irc.Conn, line *irc.Line) {
		channel, ok := client.channels[line.Args[2]]
		if !ok {
			log.Println("Invalid Channel :", line.Args[2])
			return
		}
		for _, v := range strings.Split(line.Args[3], " ") {
			nick := strings.Trim(v, "@+ ")
			channel.members[nick] = true
		}
	})

	client.conn.AddHandler("366", func(conn *irc.Conn, line *irc.Line) {
		channel, ok := client.channels[line.Args[1]]
		if !ok {
			log.Println("Invalid Channel :", line.Args[1])
			return
		}
		_channel := channel.WriteChannelInfo()

		msg := client.MakeZmqMsg("ADD_CHANNEL")
		msg.Params["channel"] = _channel
		zmqMgr.Send <- msg
	})

	client.conn.AddHandler("NICK", func(conn *irc.Conn, line *irc.Line) {
		if line.Nick == conn.Me.Nick {
			client.serverInfo.User.Nickname = line.Args[0]
			client.WriteServerInfo()
		}
		for _, channel := range client.channels {
			if channel.NickChange(line.Nick, line.Args[0]) {
				message := fmt.Sprintf("%s is now known as %s", line.Nick, line.Args[0])
				ircLog := client.WriteChatLog(line.Time, "", channel.name, message)

				msg := client.MakeZmqMsg("CHAT")
				msg.Params["log"] = ircLog
				zmqMgr.Send <- msg
			}
		}
	})

	client.conn.AddHandler("PRIVMSG", func(conn *irc.Conn, line *irc.Line) {
		log.Printf("%+v\n", line)
		if len(line.Args) == 2 && line.Args[0][0] == '#' {
			// write log to redis
			ircLog := client.WriteChatLog(line.Time, line.Nick, line.Args[0], line.Args[1])

			msg := client.MakeZmqMsg("CHAT")
			msg.Params["log"] = ircLog
			zmqMgr.Send <- msg
		}
	})
	return client
}

func (c *IRCClient) MakeZmqMsg(cmd string) *common.ZmqMsg {
	return &common.ZmqMsg{
		Cmd:      cmd,
		UserId:   c.UserId,
		ServerId: c.ServerId,
		Params:   make(map[string]interface{}),
	}
}

func (c *IRCClient) Connect() {
	addr := fmt.Sprintf("%s:%d", c.serverInfo.Server.Host, c.serverInfo.Server.Port)
	if err := c.conn.Connect(addr); err != nil {
		log.Println(err)
	}
}

func (c *IRCClient) SendLog(target, message string) {
	// write to redis for logging
	c.conn.Privmsg(target, message)
}

func (c *IRCClient) AddChannel(channel string) {
	c.channels[channel] = &Channel{
		userId:   c.UserId,
		serverId: c.ServerId,
		name:     channel,
		members:  make(map[string]bool),
	}

	r := redisPool.Get()
	r.Hset(c.ChannelKey(), c.channels[channel].ChannelKey(), []byte(strconv.Itoa(c.ServerId)))
	redisPool.Put(r)
	c.channels[channel].WriteChannelInfo()

	c.channels[channel].joined = c.conn.Connected
	if c.conn.Connected {
		c.conn.Join(channel)
	}
}

func (c *IRCClient) GetLogId() int64 {
	r := redisPool.Get()
	defer redisPool.Put(r)
	id, _ := r.Incr(fmt.Sprintf("logid:%s", c.UserId))
	return id
}

func (c *IRCClient) ChannelKey() string {
	return fmt.Sprintf("Channels:%s", c.UserId)
}

func (c *IRCClient) ServerKey() string {
	return fmt.Sprintf("Servers:%s", c.UserId)
}

func (c *IRCClient) WriteChatLog(timestamp time.Time, from, channel, message string) *common.IRCLog {
	ircLog := &common.IRCLog{
		Log_id:    c.GetLogId(),
		Timestamp: common.UnixMilli(timestamp),
		Server_id: c.ServerId,
		Channel:   channel,
		From:      from,
		Message:   message,
	}

	r := redisPool.Get()
	defer redisPool.Put(r)

	key := fmt.Sprintf("log:%s:%d:%s", c.UserId, c.ServerId, channel)
	data, _ := json.Marshal(ircLog)
	r.Lpush(key, data)

	return ircLog
}

func (c *IRCClient) WriteServerInfo() {
	r := redisPool.Get()
	defer redisPool.Put(r)

	data, _ := json.Marshal(c.serverInfo)
	err := r.Hset(c.ServerKey(), strconv.Itoa(c.ServerId), data)
	if err != nil {
		log.Println("WriteServerInfo Error:", err)
	}
}

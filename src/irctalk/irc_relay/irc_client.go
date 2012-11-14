package main

import (
	"crypto/tls"
	"fmt"
	irc "github.com/fluffle/goirc/client"
	"hash/crc32"
	"irctalk/common"
	"log"
	"strings"
	"sync"
	"time"
)

type IRCManager struct {
	sync.RWMutex
	clients  map[string]*IRCClient
	register chan *IRCClient
}

func (im *IRCManager) run() {
	for {
		select {
		case c := <-im.register:
			im.Lock()
			im.clients[c.Id] = c
			im.Unlock()
			go c.Connect()
		}
	}
}

func (im *IRCManager) GetClient(userId string, serverId int) *IRCClient {
	im.RLock()
	defer im.RUnlock()

	clientId := fmt.Sprintf("%s#%d", userId, serverId)
	c, ok := im.clients[clientId]
	if !ok {
		return nil
	}
	return c
}

func (im *IRCManager) GetClientByMsg(msg *common.ZmqMsg) *IRCClient {
	return im.GetClient(msg.UserId, msg.ServerId)
}

type IRCClient struct {
	Id           string
	UserId       string
	ServerId     int
	serverInfo   *common.IRCServer
	conn         *irc.Conn
	channels     map[string]*Channel
	disconnected chan bool
	logIdSeq     *common.RedisNumber
}

func NewClient(info *common.IRCServer) *IRCClient {
	ident := fmt.Sprintf("%0x", crc32.ChecksumIEEE([]byte(info.UserId)))
	clientId := fmt.Sprintf("%s#%d", info.UserId, info.Id)

	client := &IRCClient{
		Id:           clientId,
		UserId:       info.UserId,
		ServerId:     info.Id,
		serverInfo:   info,
		conn:         irc.SimpleClient(info.User.Nickname, ident, info.User.Realname),
		channels:     make(map[string]*Channel),
		disconnected: make(chan bool),
		logIdSeq:     &common.RedisNumber{Key: fmt.Sprintf("logid:%s", info.UserId)},
	}
	client.conn.SSL = info.Server.SSL
	client.conn.SSLConfig = &tls.Config{
		InsecureSkipVerify: true,
	}

	client.conn.AddHandler("376", func(conn *irc.Conn, line *irc.Line) {
		for _, channel := range client.channels {
			if !channel.joined {
				channel.joined = true
				conn.Join(channel.info.Name)
			}
		}

		zmqMgr.Send <- client.MakeZmqMsg(common.ZmqServerStatus{Active: true})

		client.serverInfo.Active = true
		client.WriteServerInfo()
	})

	client.conn.AddHandler("disconnected", func(conn *irc.Conn, line *irc.Line) {
		zmqMgr.Send <- client.MakeZmqMsg(common.ZmqServerStatus{Active: false})

		client.serverInfo.Active = false
		client.WriteServerInfo()
		for _, channel := range client.channels {
			channel.joined = false
			channel.info.Joined = false
			channel.WriteChannelInfo(false)
		}
		client.disconnected <- true
	})

	client.conn.AddHandler("TOPIC", func(conn *irc.Conn, line *irc.Line) {
		ircLog := client.WriteChatLog(line.Time, "", line.Args[0], fmt.Sprintf("%s changed the topic of %s to: %s", line.Nick, line.Args[0], line.Args[1]))

		zmqMgr.Send <- client.MakeZmqMsg(common.ZmqChat{Log: ircLog})

		if channel, ok := client.GetChannel(line.Args[0]); ok {
			delta := channel.SetTopic(line.Args[1])
			zmqMgr.Send <- client.MakeZmqMsg(common.ZmqUpdateChannel{DeltaChannels: []common.IRCDeltaChannel{delta}})
		} else {
			log.Println("Invalid Channel :", line.Args[0])
			return
		}
	})

	client.conn.AddHandler("JOIN", func(conn *irc.Conn, line *irc.Line) {
		ircLog := client.WriteChatLog(line.Time, "", line.Args[0], fmt.Sprintf("%s has joined %s", line.Nick, line.Args[0]))

		zmqMgr.Send <- client.MakeZmqMsg(common.ZmqChat{Log: ircLog})

		if line.Nick == conn.Me.Nick {
			// join channel by me
		} else {
			channel, ok := client.GetChannel(line.Args[0])
			if !ok {
				log.Println("Invalid Channel :", line.Args[0])
				return
			}
			delta := channel.JoinUser(line.Nick)
			zmqMgr.Send <- client.MakeZmqMsg(common.ZmqUpdateChannel{DeltaChannels: []common.IRCDeltaChannel{delta}})
		}
	})

	client.conn.AddHandler("PART", func(conn *irc.Conn, line *irc.Line) {
		ircLog := client.WriteChatLog(line.Time, "", line.Args[0], fmt.Sprintf("%s has left %s", line.Nick, line.Args[0]))

		zmqMgr.Send <- client.MakeZmqMsg(common.ZmqChat{Log: ircLog})

		channel, ok := client.GetChannel(line.Args[0])
		if !ok {
			log.Println("Invalid Channel :", line.Args[0])
			return
		}
		if delta, ok := channel.PartUser(line.Nick); ok {
			zmqMgr.Send <- client.MakeZmqMsg(common.ZmqUpdateChannel{DeltaChannels: []common.IRCDeltaChannel{delta}})
		}
	})

	client.conn.AddHandler("332", func(conn *irc.Conn, line *irc.Line) {
		channel, ok := client.GetChannel(line.Args[1])
		if !ok {
			log.Println("Invalid Channel :", line.Args[1])
			return
		}
		channel.info.Topic = line.Args[2]
	})

	client.conn.AddHandler("353", func(conn *irc.Conn, line *irc.Line) {
		channel, ok := client.GetChannel(line.Args[2])
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
		channel, ok := client.GetChannel(line.Args[1])
		if !ok {
			log.Println("Invalid Channel :", line.Args[1])
			return
		}
		channel.info.Joined = true
		delta := channel.WriteChannelInfo(true)

		zmqMgr.Send <- client.MakeZmqMsg(common.ZmqUpdateChannel{DeltaChannels: []common.IRCDeltaChannel{delta}})
	})

	client.conn.AddHandler("NICK", func(conn *irc.Conn, line *irc.Line) {
		if line.Nick == conn.Me.Nick {
			client.serverInfo.User.Nickname = line.Args[0]
			client.WriteServerInfo()
		}
		message := fmt.Sprintf("%s is now known as %s", line.Nick, line.Args[0])

		var deltaChannels []common.IRCDeltaChannel
		for _, channel := range client.channels {
			if delta, changed := channel.NickChange(line.Nick, line.Args[0]); changed {
				ircLog := client.WriteChatLog(line.Time, "", channel.info.Name, message)

				zmqMgr.Send <- client.MakeZmqMsg(common.ZmqChat{Log: ircLog})
				deltaChannels = append(deltaChannels, delta)
			}
		}
		if deltaChannels != nil {
			zmqMgr.Send <- client.MakeZmqMsg(common.ZmqUpdateChannel{DeltaChannels: deltaChannels})
		}
	})

	client.conn.AddHandler("QUIT", func(conn *irc.Conn, line *irc.Line) {
		message := fmt.Sprintf("%s has quit [%s]", line.Nick, line.Args[0])

		var deltaChannels []common.IRCDeltaChannel
		for _, channel := range client.channels {
			if delta, parted := channel.PartUser(line.Nick); parted {
				ircLog := client.WriteChatLog(line.Time, "", channel.info.Name, message)
				zmqMgr.Send <- client.MakeZmqMsg(common.ZmqChat{Log: ircLog})
				deltaChannels = append(deltaChannels, delta)
			}
		}
		if deltaChannels != nil {
			zmqMgr.Send <- client.MakeZmqMsg(common.ZmqUpdateChannel{DeltaChannels: deltaChannels})
		}
	})

	client.conn.AddHandler("PRIVMSG", func(conn *irc.Conn, line *irc.Line) {
		log.Printf("%+v\n", line)
		if len(line.Args) == 2 && line.Args[0][0] == '#' {
			// write log to redis
			ircLog := client.WriteChatLog(line.Time, line.Nick, line.Args[0], line.Args[1])
			ircLog.Noti = strings.Contains(line.Args[1], conn.Me.Nick)
			zmqMgr.Send <- client.MakeZmqMsg(common.ZmqChat{Log: ircLog})
		}
	})
	return client
}

func (c *IRCClient) MakeZmqMsg(packet common.ZmqPacket) *common.ZmqMsg {
	return common.MakeZmqMsg(c.UserId, c.ServerId, packet)
}

func (c *IRCClient) Connect() {
	for {
		addr := fmt.Sprintf("%s:%d", c.serverInfo.Server.Host, c.serverInfo.Server.Port)
		if err := c.conn.Connect(addr); err != nil {
			log.Println("Connect Error:", err)
		} else {
			<-c.disconnected
		}
		time.Sleep(10 * time.Second)
	}
}

func (c *IRCClient) SendLog(target, message string) {
	// write to redis for logging
	c.conn.Privmsg(target, message)
	c.WriteChatLog(time.Now(), c.serverInfo.User.Nickname, target, message)
}

func (c *IRCClient) AddChannel(name string) {
	channel := &Channel{
		info: &common.IRCChannel{
			UserId:   c.UserId,
			ServerId: c.ServerId,
			Name:     name,
			Members:  make([]string, 0),
		},
		members: make(map[string]bool),
	}

	channels := []*common.IRCChannel{channel.info}
	err := common.RedisSliceSave(fmt.Sprintf("channels:%s", c.UserId), &channels)

	if err != nil {
		log.Println("AddChannel Error: ", err)
		return
	}

	channel.joined = c.serverInfo.Active
	c.channels[strings.ToLower(name)] = channel
	if c.serverInfo.Active {
		c.conn.Join(name)
	}
}

func (c *IRCClient) WriteChatLog(timestamp time.Time, from, channel, message string) *common.IRCLog {
	ircLog := &common.IRCLog{
		UserId:    c.UserId,
		LogId:     c.logIdSeq.Incr(),
		Timestamp: common.UnixMilli(timestamp),
		ServerId:  c.ServerId,
		Channel:   channel,
		From:      from,
		Message:   message,
	}

	common.RedisSave(ircLog)

	return ircLog
}

func (c *IRCClient) WriteServerInfo() {
	err := common.RedisSave(c.serverInfo)

	if err != nil {
		log.Println("WriteServerInfo Error:", err)
	}
}

func (c *IRCClient) GetChannel(name string) (*Channel, bool) {
	channel, ok := c.channels[strings.ToLower(name)]
	return channel, ok
}

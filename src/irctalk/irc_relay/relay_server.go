package main

import (
	"irctalk/common"
	"log"
	"os"
	"redigo/redis"
)

var quit = make(chan bool)
var zmqMgr = common.NewZmqMessenger("tcp://127.0.0.1:9200", "tcp://127.0.0.1:9100", 4)

var ircMgr = &IRCManager{
	clients:  make(map[string]*IRCClient),
	register: make(chan *IRCClient),
}

func InitHandler() {
	zmqMgr.HandleFunc("ADD_SERVER", func(msg *common.ZmqMsg) {
		log.Println(msg)
		info := msg.Body().(*common.ZmqAddServer).ServerInfo
		log.Println(info)
		c := NewClient(info)
		ircMgr.register <- c
	})

	zmqMgr.HandleFunc("ADD_CHANNEL", func(msg *common.ZmqMsg) {
		packet := msg.Body().(*common.ZmqAddChannel)
		log.Printf("%+v", packet)
		c := ircMgr.GetClientByMsg(msg)
		c.AddChannel(packet.Channel.Name)
	})

	zmqMgr.HandleFunc("SEND_CHAT", func(msg *common.ZmqMsg) {
		packet := msg.Body().(*common.ZmqSendChat)
		log.Println(msg)
		c := ircMgr.GetClientByMsg(msg)
		c.SendLog(packet.Target, packet.Message)
	})

	zmqMgr.HandleFunc("JOIN_PART_CHANNEL", func(msg *common.ZmqMsg) {
		packet := msg.Body().(*common.ZmqJoinPartChannel)
		c := ircMgr.GetClientByMsg(msg)
		log.Println(msg, packet)
		c.JoinPartChannel(packet.Channel, packet.Join)
	})
}

func LoadDb() {
	r := common.DefaultRedisPool().Get()
	defer r.Close()

	// 서버 목록들을 읽어와서 IRCClient를 생성하고 접속
	reply, err := redis.Values(r.Do("KEYS", "servers:*"))
	if err != nil {
		log.Println("LoadDb: ", err)
		os.Exit(1)
	}
	for _, key := range reply {
		key, _ := redis.String(key, nil)
		var servers []*common.IRCServer
		err = common.RedisSliceLoad(key, &servers)
		if err != nil {
			log.Println("LoadDb: ", err)
			os.Exit(1)
		}
		for _, server := range servers {
			server.Active = false
			ircMgr.register <- NewClient(server)
		}
	}

	// 채널 목록을 읽어와서 채널을 추가
	reply, err = redis.Values(r.Do("KEYS", "channels:*"))
	if err != nil {
		log.Println("LoadDb: ", err)
		os.Exit(1)
	}
	for _, key := range reply {
		key, _ := redis.String(key, nil)
		var channels []*common.IRCChannel
		err = common.RedisSliceLoad(key, &channels)
		if err != nil {
			log.Println("LoadDb: ", err)
			os.Exit(1)
		}
		for _, channel := range channels {
			c := ircMgr.GetClient(channel.UserId, channel.ServerId)
			c.AddChannel(channel.Name)
		}
	}
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	common.MakeRedisPool("tcp", ":9002", 0, 16)
	go ircMgr.run()
	LoadDb()
	common.RegisterPacket()
	InitHandler()
	zmqMgr.Start()
	<-quit
}

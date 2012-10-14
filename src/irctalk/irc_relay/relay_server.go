package main

import (
	"irctalk/common"
	"log"
)

var quit = make(chan bool)
var zmqMgr = common.NewZmqMessenger("tcp://127.0.0.1:9200", "tcp://127.0.0.1:9100", 4)
var redisPool = common.NewRedisConnectionPool("localhost", 9002, 16)

var ircMgr = &IRCManager{
	clients:  make(map[string]*IRCClient),
	register: make(chan *IRCClient),
}

func InitHandler() {
	zmqMgr.HandleFunc("ADD_SERVER", func(msg *common.ZmqMsg) {
		log.Println(msg)
		c := NewClient(msg)
		ircMgr.register <- c
	})

	zmqMgr.HandleFunc("ADD_CHANNEL", func(msg *common.ZmqMsg) {
		log.Println(msg)
		c := ircMgr.GetClient(msg)
		c.AddChannel(msg.Params["channel"].(string))
	})

	zmqMgr.HandleFunc("SEND_CHAT", func(msg *common.ZmqMsg) {
		log.Println(msg)
		c := ircMgr.GetClient(msg)
		c.SendLog(msg.Params["target"].(string), msg.Params["message"].(string))
	})

	zmqMgr.HandleFunc("USER_ACTIVE", func(msg *common.ZmqMsg) {
		log.Println(msg)
		c := ircMgr.GetClient(msg)
		c.SetActive(msg.Params["active"].(bool))
	})
}

func main() {
	InitHandler()
	go ircMgr.run()
	zmqMgr.Start()
	<-quit
}

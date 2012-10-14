package main

import (
	"irctalk/common"
	"log"
)

var quit = make(chan bool)
var zmqMgr = common.NewZmqMessenger("tcp://127.0.0.1:9200", "tcp://127.0.0.1:9100", 4)

var ircMgr = &IRCManager {
	clients: make(map[string]*IRCClient),
	register: make(chan *IRCClient),
}

func InitHandler() {
	zmqMgr.HandleFunc("ADD_SERVER", func(msg *common.ZmqMsg) {
		log.Println(msg)
		client := NewClient(msg)
		ircMgr.register <- client
	})

	zmqMgr.HandleFunc("ADD_CHANNEL", func(msg *common.ZmqMsg) {
		log.Println(msg)
		client := ircMgr.GetClient(msg)
		client.AddChannel(msg.Params["channel"].(string))
	})

	zmqMgr.HandleFunc("SEND_CHAT", func(msg *common.ZmqMsg) {
		log.Println(msg)
		c := ircMgr.GetClient(msg)
		c.SendLog(msg.Params["target"].(string), msg.Params["message"].(string))
	})
}

func main() {
	InitHandler()
	go ircMgr.run()
	zmqMgr.Start()
	<-quit
}

package main

import (
	"code.google.com/p/go.net/websocket"
	"irctalk/common"
	"log"
	"net/http"
)

type Managers struct {
	connection *ConnectionManager
	user       *UserManager
	zmq        *common.ZmqMessenger
	push       *PushManager
}

var config common.WebsocketServer

func (m *Managers) start() {
	if err := common.InitConfig(); err != nil {
		log.Fatalln(err)
	}
	config = common.Config.WebsocketServer
	RegisterPacket()
	common.MakeRedisPool("tcp", ":9002", 0, 16)
	common.RegisterPacket()
	InitHandler(m.zmq)
	go m.push.run()
	go m.zmq.Start()
	go m.connection.run()
	go m.user.run()
}

var manager = &Managers{
	connection: &ConnectionManager{
		connections: make(map[*Connection]bool),
		register:    make(chan *Connection),
		unregister:  make(chan *Connection),
	},
	user: &UserManager{
		users:      make(map[string]*User),
		caches:     make(map[string]string),
		register:   make(chan *Connection),
		unregister: make(chan *Connection),
		broadcast:  make(chan *UserMessage, 256),
	},
	zmq: common.NewZmqMessenger("tcp://127.0.0.1:9100", "tcp://127.0.0.1:9200", 4),
	push: &PushManager{
		Send: make(chan *PushMessage, 256),
	},
}

func wsHandler(ws *websocket.Conn) {
	log.Println("connected!")
	c := &Connection{send: make(chan *Packet, 256), ws: ws, handler: manager.connection.h}
	manager.connection.register <- c
	defer func() { manager.connection.unregister <- c }()
	go c.writer()
	c.reader()
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	manager.start()
	http.Handle("/", websocket.Handler(wsHandler))
	err := http.ListenAndServe(":9001", nil)
	if err != nil {
		panic("ListenAndServe: " + err.Error())
	}
}

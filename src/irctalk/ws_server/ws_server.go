package main

import (
	"code.google.com/p/go.net/websocket"
	"irctalk/common"
	"log"
	"net/http"
	"os"
)

var logger = log.New(os.Stdout, "", log.LstdFlags|log.Lshortfile)

type Managers struct {
	connection *ConnectionManager
	user       *UserManager
	zmq        *common.ZmqMessenger
	redis      *common.RedisConnectionPool
}

func (m *Managers) start() {
	InitHandler(m.zmq)
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
	zmq:   common.NewZmqMessenger("tcp://127.0.0.1:9100", "tcp://127.0.0.1:9200", 4),
	redis: common.NewRedisConnectionPool("localhost", 9002, 16),
}

type Packet struct {
	Cmd     string                 `json:"type"`
	Msg_id  int                    `json:"msg_id,omitempty"`
	Status  int                    `json:"status"`
	Msg     string                 `json:"msg,omitempty"`
	RawData map[string]interface{} `json:"data"`
}

func (p Packet) MakeResponse() *Packet {
	resp := p
	resp.RawData = make(map[string]interface{})
	resp.Status = 0
	return &resp
}

func wsHandler(ws *websocket.Conn) {
	logger.Println("connected!")
	c := &Connection{send: make(chan *Packet, 256), ws: ws, handler: manager.connection.h}
	manager.connection.register <- c
	defer func() { manager.connection.unregister <- c }()
	go c.writer()
	c.reader()
}

func main() {
	manager.start()
	http.Handle("/", websocket.Handler(wsHandler))
	err := http.ListenAndServe(":9001", nil)
	if err != nil {
		panic("ListenAndServe: " + err.Error())
	}
}
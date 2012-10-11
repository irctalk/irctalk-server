package common

import (
	zmq "github.com/alecthomas/gozmq"
	"encoding/json"
	"log"
)

type ZmqMsg struct {
	Cmd      string
	UserId   string
	ServerId int
	Params   map[string]interface{}
}

type ZmqMessenger struct {
	ctx       zmq.Context
	sock_push zmq.Socket
	sock_pull zmq.Socket
	Send      chan *ZmqMsg
	Recv      chan *ZmqMsg
	handler   map[string]ZmqHandler
	numWorker int
}

type ZmqHandler interface {
	Handle(*ZmqMsg)
}

type ZmqHandlerFunc func(*ZmqMsg)

func (f ZmqHandlerFunc) Handle(msg *ZmqMsg) {
	f(msg)
}

// binding push address and connect pull address
func NewZmqMessenger(pushaddr, pulladdr string, numWorker int) *ZmqMessenger {
	ctx, _ := zmq.NewContext()
	sock_push, _ := ctx.NewSocket(zmq.PUSH)
	sock_pull, _ := ctx.NewSocket(zmq.PULL)

	sock_push.Bind(pushaddr)
	sock_pull.Connect(pulladdr)

	return &ZmqMessenger{
		ctx:       ctx,
		sock_push: sock_push,
		sock_pull: sock_pull,
		Send:      make(chan *ZmqMsg, 256),
		Recv:      make(chan *ZmqMsg, 256),
		handler:   make(map[string]ZmqHandler),
		numWorker: numWorker,
	}
}

func (zm *ZmqMessenger) Start() {
	// start recver
	go func() {
		for {
			msg, _ := zm.sock_pull.Recv(0)
			var zmq_msg *ZmqMsg
			json.Unmarshal(msg, zmq_msg)
			zm.Recv <- zmq_msg
		}
	}()
	// start sender
	go func() {
		for msg := range zm.Send {
			data, _ := json.Marshal(msg)
			zm.sock_push.Send(data, 0)
		}
	}()

	for i := 0; i < zm.numWorker; i++ {
		go zm.Worker()
	}
}

func (zm *ZmqMessenger) HandleFunc(cmd string, handler func(*ZmqMsg)) {
	zm.handler[cmd] = ZmqHandlerFunc(handler)
}

func (zm *ZmqMessenger) Worker() {
	for msg := range zm.Recv {
		h, ok := zm.handler[msg.Cmd]
		if !ok {
			log.Printf("Unhandled Zmq Message : %+v\n", *msg)
			continue
		}
		h.Handle(msg)
	}
}

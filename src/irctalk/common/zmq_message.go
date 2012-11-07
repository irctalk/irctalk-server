package common

import (
	"bytes"
	"encoding/gob"
	"fmt"
	zmq "github.com/alecthomas/gozmq"
	"log"
	"reflect"
)

type ZmqMsg struct {
	Cmd      string
	UserId   string
	ServerId int
	BodyType string
	RawBody  []byte
	body     interface{}
}

var typeMap map[string]reflect.Type

func (z *ZmqMsg) GetClientId() string {
	return fmt.Sprintf("%s#%d", z.UserId, z.ServerId)
}

func (z *ZmqMsg) Body() interface{} {
	return z.body
}

func MakeZmqMsg(userid string, serverid int, packet ZmqPacket) *ZmqMsg {
	return &ZmqMsg{
		Cmd:      packet.GetPacketCommand(),
		UserId:   userid,
		ServerId: serverid,
		body:     packet,
	}
}

func GobEncode(v interface{}) ([]byte, error) {
	buf := new(bytes.Buffer)
	enc := gob.NewEncoder(buf)
	err := enc.Encode(v)
	return buf.Bytes(), err
}

func GobDecode(b []byte, v interface{}) error {
	dec := gob.NewDecoder(bytes.NewBuffer(b))
	err := dec.Decode(v)
	return err
}

func (z *ZmqMsg) Decode(b []byte) error {
	if err := GobDecode(b, z); err != nil {
		return err
	}
	z.body = reflect.New(typeMap[z.BodyType]).Interface()
	return GobDecode(z.RawBody, z.body)
}

func (z *ZmqMsg) Encode() (_ []byte, err error) {
	z.BodyType = reflect.TypeOf(z.body).String()
	z.RawBody, err = GobEncode(z.body)
	if err != nil {
		return nil, err
	}
	return GobEncode(z)
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
			msg, err := zm.sock_pull.Recv(0)
			if err != nil {
				log.Println("Zmq Recv Error : ", err)
			} else {
				var zmq_msg ZmqMsg
				err := zmq_msg.Decode(msg)
				if err != nil {
					log.Println("ZmqMsg Decode Error : ", err)
				} else {
					log.Printf("%+v\n", zmq_msg)
					zm.Recv <- &zmq_msg
				}
			}
		}
	}()
	// start sender
	go func() {
		for msg := range zm.Send {
			data, err := msg.Encode()
			if err != nil {
				log.Println("ZmqMsg Encode Error : ", err)
			} else {
				zm.sock_push.Send(data, 0)
			}
		}
	}()

	for i := 0; i < zm.numWorker; i++ {
		go zm.Worker()
	}
}

func (zm *ZmqMessenger) HandleFunc(cmd string, handler func(*ZmqMsg)) {
	zm.handler[cmd] = ZmqHandlerFunc(handler)
}

func registerPacketType(v interface{}) {
	t := reflect.TypeOf(v)
	typeMap[t.String()] = t
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

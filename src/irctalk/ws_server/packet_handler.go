package main

import (
	"encoding/json"
	"fmt"
	"log"
	"reflect"
)

type PacketHandlerFunc func(*Connection, *Packet)

func (f PacketHandlerFunc) Handle(c *Connection, p *Packet) {
	defer func() {
		if err := recover(); err != nil {
			log.Println(err)
		}
	}()
	f(c, p)
}

type PacketHandler interface {
	Handle(*Connection, *Packet)
}

type WorkerJob interface {
	Run()
}

type PacketJob struct {
	c *Connection
	p *Packet
	h PacketHandler
}

func (job *PacketJob) Run() {
	job.h.Handle(job.c, job.p)
}

type PacketMux struct {
	m        map[string]PacketHandler
	job      chan WorkerJob
	n_worker int
}

func NewPacketMux() *PacketMux {
	return &PacketMux{m: make(map[string]PacketHandler), job: make(chan WorkerJob, 256), n_worker: 4}
}

var DefaultPacketMux = NewPacketMux()

func (mux *PacketMux) HandleFunc(cmd string, handler func(*Connection, *Packet)) {
	mux.m[cmd] = PacketHandlerFunc(handler)
}

func HandleFunc(cmd string, handler func(*Connection, *Packet)) {
	DefaultPacketMux.HandleFunc(cmd, handler)
}

func (mux *PacketMux) Handle(conn *Connection, packet *Packet) {
	h, h_ok := mux.m[packet.Cmd]
	if !h_ok {
		h = NotFoundHandler()
	}

	t, t_ok := reqTypeMap[packet.Cmd]
	if !t_ok {
		log.Println("Unknown PacketTypeError: ", packet.Cmd)
		return
	}
	packet.body = reflect.New(t).Interface()
	err := json.Unmarshal(packet.RawData, packet.body)
	if err != nil {
		log.Println("Json Unmarshal Error: ", packet.Cmd, err)
		return
	}
	mux.job <- &PacketJob{c: conn, p: packet, h: h}
}

func (mux *PacketMux) Worker() {
	for {
		select {
		case job := <-mux.job:
			job.Run()
		}
	}
}

func NotFoundHandler() PacketHandler {
	return PacketHandlerFunc(func(c *Connection, packet *Packet) {
		log.Println("Unhandled Packet Type :", packet.Cmd)
		resp := packet.MakeResponse()
		defer func() { c.send <- resp }()
		resp.Status = -404
		resp.Msg = fmt.Sprintf("Unhandled Packet Type : %s", packet.Cmd)
	})
}

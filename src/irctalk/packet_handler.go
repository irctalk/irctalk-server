package main

import (
	"fmt"
)

type PacketHandlerFunc func(*Connection, *Packet)

func (f PacketHandlerFunc) Handle(c *Connection, p *Packet) {
	defer func() {
		if err := recover(); err != nil {
			logger.Println(err)
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
	h, ok := mux.m[packet.Cmd]
	if !ok {
		h = NotFoundHandler()
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
		logger.Println("Unhandled Packet Type :", packet.Cmd)
		resp := packet.MakeResponse()
		defer func() { c.send <- resp }()
		resp.Status = -404
		resp.Msg = fmt.Sprintf("Unhandled Packet Type : %s", packet.Cmd)
	})
}

package common

import (
	redis "github.com/alphazero/Go-Redis"
	//	"fmt"
)

type RedisConnectionPool struct {
	spec *redis.ConnectionSpec
	Conn chan redis.Client
}

func NewRedisConnectionPool(host string, port, numConns int) *RedisConnectionPool {
	spec := redis.DefaultSpec().Host(host).Port(port)
	pool := &RedisConnectionPool{
		spec: spec,
		Conn: make(chan redis.Client, numConns),
	}
	for i := 0; i < numConns; i++ {
		conn, _ := redis.NewSynchClientWithSpec(pool.spec)
		pool.Conn <- conn
	}
	return pool
}

func (pool *RedisConnectionPool) Get() (conn redis.Client) {
	conn = <-pool.Conn
	return
}

func (pool *RedisConnectionPool) Put(conn redis.Client) {
	pool.Conn <- conn
}

func Convert(raw [][]byte) map[string]string {
	result := make(map[string]string)
	var key string
	for _, v := range raw {
		if key == "" {
			key = string(v)
		} else {
			result[key] = string(v)
			key = ""
		}
	}
	return result
}

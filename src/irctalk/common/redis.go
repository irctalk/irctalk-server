package common

import (
	//	redis "github.com/alphazero/Go-Redis"
	//	"fmt"
	"encoding/json"
)

// temporary type
type RedisConnection interface{}
type RedisConnector struct {
	write chan RedisWriter
	read  chan RedisReader
}

func (r *RedisConnector) Worker() {
	// make connection
	for {
	}
}

type RedisObject interface {
	GetRedisKey() string
}

type RedisWriter interface {
	Write(*RedisConnection)
}

type RedisReader interface {
	Read(*RedisConnection)
}

type JSONWriter struct {
	key   string
	value []byte
}

func (w *JSONWriter) Write(r *RedisConnection) {
}

func WriteRedis(v RedisObject) {
	WriteRedisWithKey(v.GetRedisKey(), v)
}

func WriteRedisWithKey(key string, v interface{}) {
	value, err := json.Marshal(v)
	if err != nil {
	}
	_ = &JSONWriter{key: key, value: value}
}

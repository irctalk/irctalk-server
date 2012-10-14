package main

import (
	"encoding/json"
	"fmt"
	"irctalk/common"
)

type Channel struct {
	joined   bool
	userId   string
	serverId int
	name     string
	members  map[string]bool
	topic    string
}

func (c *Channel) ChannelKey() string {
	return fmt.Sprintf("channel:%s:%d:%s", c.userId, c.serverId, c.name)
}

func (c *Channel) WriteChannelInfo() {
	r := redisPool.Get()
	defer redisPool.Put(r)

	channel := &common.IRCChannel{
		Server_id: c.serverId,
		Name:      c.name,
		Topic:     c.topic,
		Members:   make([]string, 0),
	}
	for k, _ := range c.members {
		channel.Members = append(channel.Members, k)
	}

	data, _ := json.Marshal(channel)
	r.Set(c.ChannelKey(), data)
}

func (c *Channel) NickChange(from, to string) bool {
	_, ok := c.members[from]
	if ok {
		delete(c.members, from)
		c.members[to] = true
		c.WriteChannelInfo()
		return true
	}
	return false
}

func (c *Channel) JoinUser(nick string) {
	c.members[nick] = true
	c.WriteChannelInfo()
}

func (c *Channel) PartUser(nick string) {
	delete(c.members, nick)
	c.WriteChannelInfo()
}

func (c *Channel) SetTopic(topic string) {
	c.topic = topic
	c.WriteChannelInfo()
}

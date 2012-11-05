package main

import (
	"encoding/json"
	"fmt"
	"irctalk/common"
	"log"
)

type Channel struct {
	joined   bool
	members  map[string]bool
	info	 *common.IRCChannel
}

func (c *Channel) WriteChannelInfo(memberChanged bool) *common.IRCChannel {
	if memberChanged {
		c.info.Members = make([]string, len(members))
		i := 0
		for nick, _ := range members {
			info.Members[i] = nick
			i++
		}
	}

	if err := common.RedisSave(c.info); err != nil {
		log.Println("WriteChannelInfo Error : ", err)
	}

	return c.info
}

func (c *Channel) NickChange(from, to string) bool {
	_, ok := c.members[from]
	if ok {
		delete(c.members, from)
		c.members[to] = true
		c.WriteChannelInfo(true)
		return true
	}
	return false
}

func (c *Channel) JoinUser(nick string) {
	c.members[nick] = true
	c.WriteChannelInfo(true)
}

func (c *Channel) PartUser(nick string) {
	delete(c.members, nick)
	c.WriteChannelInfo(true)
}

func (c *Channel) SetTopic(topic string) {
	c.topic = topic
	c.WriteChannelInfo(false)
}

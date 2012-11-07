package main

import (
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
		c.info.Members = make([]string, len(c.members))
		i := 0
		for nick, _ := range c.members {
			c.info.Members[i] = nick
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

func (c *Channel) PartUser(nick string) bool {
	_, ok := c.members[nick]
	if ok {
		delete(c.members, nick)
		c.WriteChannelInfo(true)
		return true
	}
	return false
}

func (c *Channel) SetTopic(topic string) {
	c.info.Topic = topic
	c.WriteChannelInfo(false)
}

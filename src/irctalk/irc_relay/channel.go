package main

import (
	"irctalk/common"
	"log"
)

type Channel struct {
	joined  bool
	members map[string]bool
	info    *common.IRCChannel
}

func (c *Channel) WriteChannelInfo(memberChanged bool) common.IRCDeltaChannel {
	delta := c.MakeDeltaChannel()
	delta["topic"] = c.info.Topic
	delta["joined"] = c.info.Joined
	if memberChanged {
		members := make([]string, len(c.members))
		c.info.Members = make([]string, len(c.members))
		i := 0
		for nick, _ := range c.members {
			c.info.Members[i] = nick
			members[i] = "+" + nick
			i++
		}
		delta["members"] = members
	}

	if err := common.RedisSave(c.info); err != nil {
		log.Println("WriteChannelInfo Error : ", err)
	}

	return delta
}

func (c *Channel) NickChange(from, to string) (common.IRCDeltaChannel, bool) {
	_, ok := c.members[from]
	if ok {
		delete(c.members, from)
		c.members[to] = true
		c.WriteChannelInfo(true)
		delta := c.MakeDeltaChannel()
		delta["members"] = []string{"+" + to, "-" + from}
		return delta, true
	}
	return nil, false
}

func (c *Channel) JoinUser(nick string) common.IRCDeltaChannel {
	c.members[nick] = true
	c.WriteChannelInfo(true)
	delta := c.MakeDeltaChannel()
	delta["members"] = []string{"+" + nick}
	return delta
}

func (c *Channel) PartUser(nick string) (common.IRCDeltaChannel, bool) {
	_, ok := c.members[nick]
	if ok {
		delete(c.members, nick)
		c.WriteChannelInfo(true)
		delta := c.MakeDeltaChannel()
		delta["members"] = []string{"-" + nick}
		return delta, true
	}
	return nil, false
}

func (c *Channel) SetTopic(topic string) common.IRCDeltaChannel {
	c.info.Topic = topic
	c.WriteChannelInfo(false)
	delta := c.MakeDeltaChannel()
	delta["topic"] = topic
	return delta
}

func (c *Channel) MakeDeltaChannel() common.IRCDeltaChannel {
	delta := make(common.IRCDeltaChannel)
	delta["server_id"] = c.info.ServerId
	delta["channel"] = c.info.Name
	return delta
}

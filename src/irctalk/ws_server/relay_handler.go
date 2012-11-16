package main

import (
	"irctalk/common"
	"log"
)

// relay server -> irctalk server 
// chat message와 같은 실시간 푸시 메세지만 전달한다.

func InitHandler(z *common.ZmqMessenger) {
	z.HandleFunc("CHAT", func(msg *common.ZmqMsg) {
		irclog := msg.Body().(*common.ZmqChat).Log

		if irclog.Noti {
			packet := MakePacket(&SendPushLog{Log: irclog})
			err := manager.user.SendPushMessage(msg.UserId, packet)
			if err != nil {
				log.Println("Push Error: ", err)
			}
		} else {
			user, err := manager.user.GetConnectedUser(msg.UserId)
			if err != nil {
				return
			}
			log.Printf("Msg Recv: %+v\n", irclog)
			packet := MakePacket(&SendPushLog{Log: irclog})
			user.Send(packet, nil)
		}
	})

	z.HandleFunc("SERVER_STATUS", func(msg *common.ZmqMsg) {
		user, err := manager.user.GetConnectedUser(msg.UserId)
		if err != nil {
			log.Println("[ZMQMSG]SERVER_STATUS ERROR:", err)
			return
		}
		active := msg.Body().(*common.ZmqServerStatus).Active
		user.ChangeServerActive(msg.ServerId, active)
	})

	z.HandleFunc("UPDATE_CHANNEL", func(msg *common.ZmqMsg) {
		user, err := manager.user.GetConnectedUser(msg.UserId)
		if err != nil {
			return
		}

		channels := msg.Body().(*common.ZmqUpdateChannel).DeltaChannels
		packet := MakePacket(&SendUpdateChannel{DeltaChannels: channels})
		user.Send(packet, nil)
	})

	z.HandleFunc("DEL_CHANNEL", func(msg *common.ZmqMsg) {
		user, err := manager.user.GetConnectedUser(msg.UserId)
		if err != nil {
			return
		}

		channel := make(common.IRCDeltaChannel)
		channel["server_id"] = msg.ServerId
		channel["channel"] = msg.Body().(*common.ZmqDelChannel).Channel
		packet := MakePacket(&SendUpdateChannel{DeltaChannels: []common.IRCDeltaChannel{channel}})
		user.Send(packet, nil)
	})
}

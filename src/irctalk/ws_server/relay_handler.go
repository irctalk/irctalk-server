package main

import (
	"irctalk/common"
)

// relay server -> irctalk server 
// chat message와 같은 실시간 푸시 메세지만 전달한다.

func InitHandler(z *common.ZmqMessenger) {
	z.HandleFunc("CHAT", func(msg *common.ZmqMsg) {
		user, err := manager.user.GetConnectedUser(msg.UserId)
		if err != nil {
			return
		}

		var irclog common.IRCLog
		common.Import(msg.Params["log"], &irclog)

		logger.Printf("Msg Recv: %+v\n", irclog)
		packet := &Packet{Cmd: "pushLog", RawData: map[string]interface{}{"log": irclog}}
		user.Send(packet, nil)
	})

	z.HandleFunc("SERVER_STATUS", func(msg *common.ZmqMsg) {
		user, err := manager.user.GetConnectedUser(msg.UserId)
		if err != nil {
			logger.Println("[ZMQMSG]SERVER_STATUS ERROR:", err)
			return
		}
		active := msg.Params["active"].(bool)
		user.ChangeServerActive(msg.ServerId, active)
	})

	z.HandleFunc("ADD_CHANNEL", func(msg *common.ZmqMsg) {
		user, err := manager.user.GetConnectedUser(msg.UserId)
		if err != nil {
			logger.Println("[ZMQMSG]SERVER_STATUS ERROR:", err)
			return
		}
		var channel common.IRCChannel
		common.Import(msg.Params["channel"], &channel)
		packet := &Packet{Cmd: "addChannel", RawData: map[string]interface{}{"channel": channel}}
		user.Send(packet, nil)
	})
}

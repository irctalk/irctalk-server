package main

import (
	"encoding/json"
	"fmt"
	gcm "github.com/googollee/go-gcm"
	"irctalk/common"
	"log"
	"strings"
)

type PushManager struct {
	Send chan *PushMessage
}

type PushMessage struct {
	UserId     string
	PushTokens []string
	Payload    *Packet
}

func (m *PushManager) run() {
	for pushMsg := range m.Send {
		_tokens := make(map[string][]string)
		for _, pushToken := range pushMsg.PushTokens {
			s := strings.SplitN(pushToken, ":", 2)
			_tokens[s[0]] = append(_tokens[s[0]], s[1])
		}
		for pushType, pushTokens := range _tokens {
			switch pushType {
			case "gcm":
				go m.SendGCM(pushMsg.UserId, pushTokens, pushMsg.Payload)
			case "apns":
				go m.SendAPNS(pushMsg.UserId, pushTokens, pushMsg.Payload)
			default:
				log.Println("Unknown Push Type Error: ", pushType)
			}
		}
	}
}

func (m *PushManager) PushTokenListKey(userId string) string {
	return fmt.Sprintf("tokens:%s", userId)
}

func (m *PushManager) SendGCM(userId string, pushTokens []string, payload *Packet) {
	c := gcm.New(common.Config.GCMAPIKey)
	msg := gcm.NewMessage(pushTokens...)
	switch body := payload.body.(type) {
	case SendPushLog:
		msg.CollapseKey = fmt.Sprintf("%d:%s", body.Log.ServerId, body.Log.Channel)
	}

	var err error
	if payload.RawData == nil {
		payload.RawData, err = json.Marshal(payload.body)
		if err != nil {
			log.Println("SendGCM Error: ", err)
			return
		}
	}

	msg.SetPayload("type", payload.Cmd)
	msg.SetPayload("data", string(payload.RawData))
	resp, err2 := c.Send(msg)
	if err2 != nil {
		log.Println("SendGCM Error: ", err2)
	}
	log.Printf("%+v", resp)
	var removeTokens []string
	for i, result := range resp.Results {
		switch result.Error {
		case "NotRegistered":
			removeTokens = append(removeTokens, "gcm:"+pushTokens[i])
		}
	}
	if removeTokens != nil {
		err = common.RedisSliceRemove(m.PushTokenListKey(userId), &removeTokens)
		if err != nil {
			log.Println("SendGCM - RemoveTokens Error: ", err2)
		}
	}
}

func (m *PushManager) SendAPNS(userId string, pushTokens []string, payload *Packet) {
	// 아직 지원하지 않습니다.
}

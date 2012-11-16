package common

import (
	"encoding/json"
	"io/ioutil"
	"log"
)

type OauthType struct {
	ClientId     string
	ClientSecret string
	TokenURL     string
	RedirectURL  string
}

type RelayServerConfig struct {
	ReconnectCount	int
	ReconnectInterval int
}
type ConfigType struct {
	Oauth               OauthType
	GCMAPIKey           string
	PushResponseTimeout int64
	RelayServer         RelayServerConfig
}

var Config ConfigType

func InitConfig() error {
	data, err := ioutil.ReadFile("config.json")
	if err != nil {
		log.Println(err)
		return err
	}
	err = json.Unmarshal(data, &Config)
	if err != nil {
		log.Println(err)
	}
	return err
}

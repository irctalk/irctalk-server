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

type ConfigType struct {
	Oauth OauthType
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

package main

import (
	"code.google.com/p/goauth2/oauth"
	"encoding/json"
	"io"
)

type GoogleOauth struct {
	config *oauth.Config
	token  string
}

type InvalidTokenError struct {
}

func (e *InvalidTokenError) Error() string {
	return "Access Token is not set! Call 'GetToken' first"
}

type InvalidCodeError struct {
}

func (e *InvalidCodeError) Error() string {
	return "Invalid Code!"
}

func NewGoogleOauth(token string) *GoogleOauth {
	g := &GoogleOauth{
		config: &oauth.Config{
			ClientId:     "812906460657-2ppupr0380a41ffau4sdcdpgcarja17c.apps.googleusercontent.com",
			ClientSecret: "ns4SeGI9TfYCvDPKilCslNKL",
			TokenURL:     "https://accounts.google.com/o/oauth2/token",
			RedirectURL:  "http://localhost",
		},
		token: token,
	}
	return g
}

func (g *GoogleOauth) GetToken(code string) (string, error) {
	t := &oauth.Transport{Config: g.config}
	if g.token != "" {
		return g.token, nil
	}
	tok, err := t.Exchange(code)
	if err != nil {
		return "", err
	}
	if tok.AccessToken == "" {
		return "", &InvalidCodeError{}
	}
	g.token = tok.AccessToken
	return g.token, nil
}

func (g *GoogleOauth) GetUserInfo() (map[string]interface{}, error) {
	var response map[string]interface{}
	t := &oauth.Transport{Config: g.config}
	if g.token == "" {
		return nil, &InvalidTokenError{}
	}
	t.Token = &oauth.Token{AccessToken: g.token}
	r, err := t.Client().Get("https://www.googleapis.com/oauth2/v1/userinfo")
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(r.Body)
	defer r.Body.Close()
	for {
		if err := dec.Decode(&response); err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}
	}
	return response, err
}

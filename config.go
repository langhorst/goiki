package main

import (
	"github.com/BurntSushi/toml"
	"io/ioutil"
)

type user struct {
	Name     string
	Email    string
	Username string
	Password string
}

type config struct {
	Name        string
	Host        string
	Port        int
	DataDir     string `toml:"data_dir"`
	TemplateDir string `toml:"template_dir"`
	Users       []user
}

func loadConfig(filename string) (config, error) {
	var c config
	var data []byte
	var err error

	if data, err = ioutil.ReadFile(filename); err != nil {
		return c, err
	}
	if _, err = toml.Decode(string(data), &c); err != nil {
		return c, err
	}
	return c, nil
}

func loadAuth(users []user) map[string]user {
	auth := make(map[string]user, len(users))
	for _, user := range users {
		auth[user.Username] = user
	}
	return auth
}

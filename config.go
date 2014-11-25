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
	Auth        map[string]user
}

func (c *config) loadAuth() {
	auth := make(map[string]user, len(c.Users))
	for _, user := range c.Users {
		auth[user.Username] = user
	}
	c.Auth = auth
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

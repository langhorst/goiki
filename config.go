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
	Name          string
	Host          string
	Port          int
	DataDir       string `toml:"data_dir"`
	IndexPage     string `toml:"index_page"`
	FileExtension string `toml:"file_extension"`
	TemplateDir   string `toml:"template_dir"`
	StaticDir     string `toml:"static_dir"`
	TableClass    string `toml:"table_class"`
	Users         []user
	Auth          map[string]user
}

func (c *config) loadAuth() {
	auth := make(map[string]user, len(c.Users))
	for _, user := range c.Users {
		auth[user.Username] = user
	}
	c.Auth = auth
}

func loadConfigFromFile(file string) (config, error) {
	var c config
	data, err := ioutil.ReadFile(file)
	if err != nil {
		return c, err
	}
	return loadConfig(string(data))
}

func loadConfig(data string) (config, error) {
	var c config
	if _, err := toml.Decode(data, &c); err != nil {
		return c, err
	}
	return c, nil
}

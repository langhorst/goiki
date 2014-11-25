package main

import (
	"testing"
)

var testConfigFile string = "./goiki.toml"

func TestConfigName(t *testing.T) {
	c, _ := loadConfig(testConfigFile)
	name := "Goiki"
	if c.Name != name {
		t.Errorf("Name should equal >%s<, but is >%s<", name, c.Name)
	}
}

func TestConfigHost(t *testing.T) {
	c, _ := loadConfig(testConfigFile)
	host := "0.0.0.0"
	if c.Host != host {
		t.Errorf("Host should equal >%s<, but is >%s<", host, c.Host)
	}
}

func TestConfigPort(t *testing.T) {
	c, _ := loadConfig(testConfigFile)
	port := 4567
	if c.Port != port {
		t.Errorf("Port should equal >%d<, but is >%d<", port, c.Port)
	}
}

func TestConfigDataDir(t *testing.T) {
	c, _ := loadConfig(testConfigFile)
	dataDir := "./data"
	if c.DataDir != dataDir {
		t.Errorf("DataDir should equal >%s<, but is >%s<", dataDir, c.DataDir)
	}
}

func TestConfigTemplateDir(t *testing.T) {
	c, _ := loadConfig(testConfigFile)
	templateDir := ""
	if c.TemplateDir != templateDir {
		t.Errorf("TemplateDir should equal >%s<, but is >%s<", templateDir, c.TemplateDir)
	}
}

func TestConfigUsers(t *testing.T) {
	c, _ := loadConfig(testConfigFile)
	u := user{Name: "Goiki", Email: "goiki@example.com", Username: "goiki", Password: "{SHA}4v0+mLtvlX3qyy5ISrQU5mw0Yhg="}

	if len(c.Users) != 1 {
		t.Errorf("Number of default users should equal to >1<, but is >%d<", len(c.Users))
	}

	if c.Users[0].Name != u.Name {
		t.Errorf("User Name should equal >%s<, but is >%s<", u.Name, c.Users[0].Name)
	}

	if c.Users[0].Email != u.Email {
		t.Errorf("User Email should equal >%s<, but is >%s<", u.Email, c.Users[0].Email)
	}

	if c.Users[0].Username != u.Username {
		t.Errorf("User Username should equal >%s<, but is >%s<", u.Username, c.Users[0].Username)
	}

	if c.Users[0].Password != u.Password {
		t.Errorf("User Password should euqual >%s<, but is >%s<", u.Password, c.Users[0].Password)
	}
}

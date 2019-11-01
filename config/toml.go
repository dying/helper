package config

import (
	"io/ioutil"

	"github.com/BurntSushi/toml"
)

func loadTomlConfig() {
	raw, err := ioutil.ReadFile("config.toml")
	if err != nil {
		panic(err)
	}

	_, err = toml.Decode(string(raw), &Conf)
	if err != nil {
		panic(err)
	}
}

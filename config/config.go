package config

import (
	"os"
)

type Config struct {
	BotToken string `toml:"botToken"`
}

type Provider int

const (
	Toml    Provider = iota
	EnvVars Provider = iota
)

var Conf Config

// LoadConfig load config depending on the provider
func (p *Provider) LoadConfig() {
	switch *p {
	case Toml:
		loadTomlConfig()
	case EnvVars:
		loadEnvVarConfig()
	}
}

// GetConfigProvider check if toml is here, or use env
func GetConfigProvider() Provider {
	tomlExists := false
	if _, err := os.Stat("config.toml"); err == nil {
		tomlExists = true
	}

	if tomlExists {
		return Toml
	} else {
		return EnvVars
	}
}

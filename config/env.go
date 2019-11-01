package config

import (
	"github.com/kelseyhightower/envconfig"
)

func loadEnvVarConfig() {
	if err := envconfig.Process("helper", &Conf); err != nil {
		panic(err)
	}
}

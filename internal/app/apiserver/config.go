package apiserver

import "github.com/AVGsync/study_flow_api/internal/database"

type Config struct {
	BindAddr string           `toml:"bind_addr"`
	LogLevel string           `toml:"log_level"`
	DB       *database.Config `toml:"db"`
	JWTSecret string          `toml:"jwt_secret"`
}

func NewConfig() *Config {
	return &Config{
		BindAddr: ":8080",
		LogLevel: "debug",
		DB:       database.NewConfig(),
	}
}

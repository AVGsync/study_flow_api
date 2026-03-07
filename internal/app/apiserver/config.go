package apiserver

import (
	"time"

	"github.com/AVGsync/study_flow_api/internal/database"
)

type Config struct {
	BindAddr string           `toml:"bind_addr"`
	RedisAddr string          `toml:"redis_addr"`
	CacheTTL  time.Duration   `toml:"cache_ttl"`
	LogLevel string           `toml:"log_level"`
	DB       *database.Config `toml:"db"`
	JWTSecret string          `toml:"jwt_secret"`
}

func NewConfig() *Config {
	return &Config{
		BindAddr: ":8080",
		RedisAddr: "localhost:6379",
		CacheTTL: 300 * time.Second,
		LogLevel: "debug",
		DB:       database.NewConfig(),
	}
}

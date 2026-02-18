package config

import (
	"fmt"
	"os"
)

type Config struct {
	BotToken string
	DBPath   string
}

func Load() (*Config, error) {
	token := os.Getenv("BOT_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("BOT_TOKEN environment variable is required")
	}
	return &Config{
		BotToken: token,
		DBPath:   getEnvOrDefault("DB_PATH", "slotbot.db"),
	}, nil
}

func getEnvOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

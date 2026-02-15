package config

import "os"

type Config struct {
	BotToken string
	DBPath   string
}

func Load() *Config {
	return &Config{
		BotToken: os.Getenv("BOT_TOKEN"),
		DBPath:   getEnvOrDefault("DB_PATH", "slotbot.db"),
	}
}

func getEnvOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

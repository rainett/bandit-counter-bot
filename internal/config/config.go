package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	BotToken string
	DBPath   string
	DevIDs   []int64
}

func Load() (*Config, error) {
	token := os.Getenv("BOT_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("BOT_TOKEN environment variable is required")
	}
	return &Config{
		BotToken: token,
		DBPath:   getEnvOrDefault("DB_PATH", "slotbot.db"),
		DevIDs:   parseDevIDs(os.Getenv("DEV_IDS")),
	}, nil
}

func parseDevIDs(raw string) []int64 {
	if raw == "" {
		return nil
	}
	var ids []int64
	for _, s := range strings.Split(raw, ",") {
		s = strings.TrimSpace(s)
		if id, err := strconv.ParseInt(s, 10, 64); err == nil {
			ids = append(ids, id)
		}
	}
	return ids
}

func getEnvOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

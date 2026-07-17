package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

const defaultDatabaseURL = "postgres://postgres:postgres@localhost:5432/notifier?sslmode=disable"

type Config struct {
	GeminiKey   string
	TelegramKey string
	DatabaseURL string
}

func Load() (*Config, error) {
	for _, path := range []string{".env.local", ".env"} {
		if err := loadDotenv(path); err != nil {
			return nil, fmt.Errorf("config: loading %s: %w", path, err)
		}
	}

	cfg := &Config{
		GeminiKey:   os.Getenv("GEMINI_KEY"),
		TelegramKey: os.Getenv("TELEGRAM_KEY"),
		DatabaseURL: envOr("DATABASE_URL", defaultDatabaseURL),
	}

	for name, value := range map[string]string{
		"GEMINI_KEY":   cfg.GeminiKey,
		"TELEGRAM_KEY": cfg.TelegramKey,
	} {
		if value == "" {
			return nil, fmt.Errorf("config: %s is not set (add it to .env.local for local runs)", name)
		}
	}

	return cfg, nil
}

func loadDotenv(path string) error {
	if _, err := os.Stat(path); err != nil {
		return nil
	}
	return godotenv.Load(path)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

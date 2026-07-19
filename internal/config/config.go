package config

import (
	"fmt"
	"net/url"
	"os"

	"github.com/joho/godotenv"
)

const defaultDatabaseURL = "postgres://postgres:postgres@localhost:5432/notifier?sslmode=disable"

type Config struct {
	GeminiKey   string
	TelegramKey string
	DatabaseURL string
	LogLevel    string
	LogFormat   string
	HealthAddr  string
}

func Load() (*Config, error) {
	if err := loadEnvFiles(); err != nil {
		return nil, err
	}

	cfg := &Config{
		GeminiKey:   os.Getenv("GEMINI_KEY"),
		TelegramKey: os.Getenv("TELEGRAM_KEY"),
		DatabaseURL: envOr("DATABASE_URL", defaultDatabaseURL),
		LogLevel:    envOr("LOG_LEVEL", "info"),
		LogFormat:   envOr("LOG_FORMAT", "console"),
		HealthAddr:  envOr("HEALTH_ADDR", ":8080"),
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

// SafeDatabaseURL returns DatabaseURL with the password masked, so connection
// details can be logged without leaking the credential.
func (c *Config) SafeDatabaseURL() string {
	return RedactURL(c.DatabaseURL)
}

// RedactURL masks the password in a database URL, leaving the rest readable.
func RedactURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		// Not parseable, so we cannot tell where the password ends; say nothing.
		return "(unparseable)"
	}
	if _, hasPassword := parsed.User.Password(); hasPassword {
		parsed.User = url.UserPassword(parsed.User.Username(), "xxxxx")
	}
	return parsed.String()
}

func LoadDatabaseURL() (string, error) {
	if err := loadEnvFiles(); err != nil {
		return "", err
	}
	return envOr("DATABASE_URL", defaultDatabaseURL), nil
}

func loadEnvFiles() error {
	for _, path := range []string{".env.local", ".env"} {
		if err := loadDotenv(path); err != nil {
			return fmt.Errorf("config: loading %s: %w", path, err)
		}
	}
	return nil
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

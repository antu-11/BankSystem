// Package config handles application-wide configuration via environment variables.
package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Config holds all runtime configuration for OmniLedger.
type Config struct {
	DatabaseURL   string
	RedisURL      string
	RedisPassword string
	RedisDB       int
	Port          string
	Env           string
	JWTSecret     string
	CORSOrigin    string

	SMTPHost     string
	SMTPPort     string
	SMTPUsername string
	SMTPPassword string
	SMTPFrom     string
}

// Load reads from .env (if present) and returns a populated Config.
func Load() (*Config, error) {
	// Best-effort .env load — fine to fail in production.
	_ = godotenv.Load()

	redisDB, _ := strconv.Atoi(getEnv("REDIS_DB", "0"))

	cfg := &Config{
		DatabaseURL:   getEnv("DATABASE_URL", ""),
		RedisURL:      getEnv("REDIS_URL", "localhost:6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),
		RedisDB:       redisDB,
		Port:          getEnv("PORT", "8080"),
		Env:           getEnv("ENV", "development"),
		JWTSecret:     getEnv("JWT_SECRET", "change-me-in-production"),
		CORSOrigin:    getEnv("CORS_ORIGIN", "http://localhost:3000"),
		SMTPHost:      getEnv("SMTP_HOST", "smtp.gmail.com"),
		SMTPPort:      getEnv("SMTP_PORT", "587"),
		SMTPUsername:  getEnv("SMTP_USERNAME", ""),
		SMTPPassword:  getEnv("SMTP_PASSWORD", ""),
		SMTPFrom:      getEnv("SMTP_FROM", "noreply@thevault.dev"),
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

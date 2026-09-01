package config

import (
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Port           string
	DatabaseURL    string
	MaxUploadBytes int64
	AllowedOrigins []string
	LogLevel       string
}

func Load() Config {
	return Config{
		Port: env("PORT", "8080"),
		DatabaseURL: env("DATABASE_URL",
			"postgres://smartgrid:smartgrid@localhost:5432/smartgrid?sslmode=disable"),
		MaxUploadBytes: int64(envInt("MAX_UPLOAD_MB", 25)) << 20,
		AllowedOrigins: strings.Split(env("ALLOWED_ORIGINS", "http://localhost:5173,http://localhost:8081"), ","),
		LogLevel:       env("LOG_LEVEL", "info"),
	}
}

func env(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v, err := strconv.Atoi(env(key, "")); err == nil && v > 0 {
		return v
	}
	return fallback
}

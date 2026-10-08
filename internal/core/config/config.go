package config

import (
	"os"
	"time"
)

type Config struct {
	HTTPPort                  string
	DatabaseURL               string
	OrderTimeoutCheckInterval time.Duration
}

func Load() *Config {
	dbURL := getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/food_delivery?sslmode=disable")
	httpPort := getEnv("CORE_HTTP_PORT", "8080")

	return &Config{
		HTTPPort:                  httpPort,
		DatabaseURL:               dbURL,
		OrderTimeoutCheckInterval: 10 * time.Second,
	}
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists && value != "" {
		return value
	}
	return fallback
}

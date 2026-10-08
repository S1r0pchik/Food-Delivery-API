package config

import (
	"os"
	"strconv"
)

type Config struct {
	HTTPPort      string
	CoreBaseURL   string
	RestaurantID  string
	APIKey        string
	WebhookSecret string
	AutoAccept    bool
}

func Load() *Config {
	autoAccept := true
	if val := os.Getenv("RESTAURANT_AUTO_ACCEPT"); val != "" {
		if parsed, err := strconv.ParseBool(val); err == nil {
			autoAccept = parsed
		}
	}

	return &Config{
		HTTPPort:      getEnv("RESTAURANT_HTTP_PORT", "8081"),
		CoreBaseURL:   getEnv("CORE_BASE_URL", "http://localhost:8080"),
		RestaurantID:  getEnv("RESTAURANT_ID", "11111111-1111-1111-1111-111111111111"),
		APIKey:        getEnv("RESTAURANT_API_KEY", "restaurant-secret-api-key-123"),
		WebhookSecret: getEnv("RESTAURANT_WEBHOOK_SECRET", "hmac-secret-signature-key-456"),
		AutoAccept:    autoAccept,
	}
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

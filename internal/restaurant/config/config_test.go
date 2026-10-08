package config_test

import (
	"os"
	"testing"

	"food-delivery-api/internal/restaurant/config"
)

func TestRestaurantConfig_Load(t *testing.T) {
	os.Setenv("RESTAURANT_AUTO_ACCEPT", "false")
	defer os.Unsetenv("RESTAURANT_AUTO_ACCEPT")

	cfg := config.Load()
	if cfg.AutoAccept {
		t.Errorf("expected AutoAccept false")
	}
}

package config_test

import (
	"os"
	"testing"

	"food-delivery-api/internal/core/config"
)

func TestCoreConfig_Load(t *testing.T) {
	os.Setenv("CORE_HTTP_PORT", "9999")
	defer os.Unsetenv("CORE_HTTP_PORT")

	cfg := config.Load()
	if cfg.HTTPPort != "9999" {
		t.Errorf("expected port 9999, got %s", cfg.HTTPPort)
	}
}

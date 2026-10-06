package config_test

import (
	"strings"
	"testing"
	"time"

	"github.com/FedorOnpoint/track_app/backend/internal/platform/config"
)

func TestLoadRequiresEnvironmentAndDatabaseURL(t *testing.T) {
	t.Parallel()

	_, err := config.Load(func(string) string { return "" })

	if err == nil {
		t.Fatal("Load() error = nil, want missing configuration error")
	}
	for _, name := range []string{"APP_ENV", "DATABASE_URL"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("Load() error = %q, want it to mention %s", err, name)
		}
	}
}

func TestLoadAppliesDevelopmentDefaults(t *testing.T) {
	t.Parallel()

	values := map[string]string{
		"APP_ENV":      "development",
		"DATABASE_URL": "postgres://tracker:tracker@postgres:5432/tracker?sslmode=disable",
	}

	got, err := config.Load(func(key string) string { return values[key] })
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if got.Addr != ":8080" {
		t.Errorf("Addr = %q, want :8080", got.Addr)
	}
	if got.SessionTTL != 168*time.Hour {
		t.Errorf("SessionTTL = %v, want 168h", got.SessionTTL)
	}
	if got.LogLevel != "info" {
		t.Errorf("LogLevel = %q, want info", got.LogLevel)
	}
}

func TestLoadRequiresDomainInProduction(t *testing.T) {
	t.Parallel()

	values := map[string]string{
		"APP_ENV":      "production",
		"DATABASE_URL": "postgres://tracker:tracker@postgres:5432/tracker",
	}

	_, err := config.Load(func(key string) string { return values[key] })

	if err == nil || !strings.Contains(err.Error(), "APP_DOMAIN") {
		t.Fatalf("Load() error = %v, want missing APP_DOMAIN error", err)
	}
}

func TestLoadRejectsInvalidSessionTTL(t *testing.T) {
	t.Parallel()

	values := map[string]string{
		"APP_ENV":      "development",
		"DATABASE_URL": "postgres://tracker:tracker@postgres:5432/tracker?sslmode=disable",
		"SESSION_TTL":  "forever",
	}

	_, err := config.Load(func(key string) string { return values[key] })

	if err == nil || !strings.Contains(err.Error(), "SESSION_TTL") {
		t.Fatalf("Load() error = %v, want invalid SESSION_TTL error", err)
	}
}

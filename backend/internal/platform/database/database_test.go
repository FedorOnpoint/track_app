package database_test

import (
	"strings"
	"testing"
	"time"

	"github.com/FedorOnpoint/track_app/backend/internal/platform/database"
)

func TestParsePoolConfigAppliesResourceConsciousLimits(t *testing.T) {
	t.Parallel()

	got, err := database.ParsePoolConfig("postgres://tracker:tracker@localhost:5432/tracker?sslmode=disable")
	if err != nil {
		t.Fatalf("ParsePoolConfig() error = %v", err)
	}

	if got.MaxConns != 10 {
		t.Errorf("MaxConns = %d, want 10", got.MaxConns)
	}
	if got.MinIdleConns != 1 {
		t.Errorf("MinIdleConns = %d, want 1", got.MinIdleConns)
	}
	if got.PingTimeout != 2*time.Second {
		t.Errorf("PingTimeout = %v, want 2s", got.PingTimeout)
	}
}

func TestParsePoolConfigRejectsInvalidURL(t *testing.T) {
	t.Parallel()

	_, err := database.ParsePoolConfig("postgres://tracker:secret-value@%zz/tracker")
	if err == nil {
		t.Fatal("ParsePoolConfig() error = nil, want invalid URL error")
	}
	if strings.Contains(err.Error(), "secret-value") {
		t.Fatalf("ParsePoolConfig() error exposes password: %v", err)
	}
}

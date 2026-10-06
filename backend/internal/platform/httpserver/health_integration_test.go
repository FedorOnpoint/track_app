//go:build integration

package httpserver_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/FedorOnpoint/track_app/backend/internal/platform/database"
	"github.com/FedorOnpoint/track_app/backend/internal/platform/httpserver"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestReadinessTracksPostgresAvailability(t *testing.T) {
	ctx := context.Background()
	container, err := postgres.Run(ctx,
		"postgres:18.6-alpine",
		postgres.WithDatabase("tracker"),
		postgres.WithUsername("tracker"),
		postgres.WithPassword("integration-only"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Fatalf("start PostgreSQL container: %v", err)
	}
	testcontainers.CleanupContainer(t, container)

	databaseURL, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("PostgreSQL connection string: %v", err)
	}
	pool, err := database.Open(ctx, databaseURL)
	if err != nil {
		t.Fatalf("open database pool: %v", err)
	}

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	handler := httpserver.NewHandler(pool, logger, httpserver.SecurityConfig{Environment: "development", Domain: "localhost"})

	ready := httptest.NewRecorder()
	handler.ServeHTTP(ready, httptest.NewRequest(http.MethodGet, "/api/v1/health/ready", nil))
	if ready.Code != http.StatusOK {
		t.Fatalf("ready status = %d, want %d", ready.Code, http.StatusOK)
	}

	pool.Close()
	unavailable := httptest.NewRecorder()
	handler.ServeHTTP(unavailable, httptest.NewRequest(http.MethodGet, "/api/v1/health/ready", nil))
	if unavailable.Code != http.StatusServiceUnavailable {
		t.Fatalf("status after pool close = %d, want %d", unavailable.Code, http.StatusServiceUnavailable)
	}
}

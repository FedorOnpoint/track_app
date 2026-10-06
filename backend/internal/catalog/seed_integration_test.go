//go:build integration

package catalog_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/FedorOnpoint/track_app/backend/internal/catalog/catalogdb"
	"github.com/FedorOnpoint/track_app/backend/internal/platform/database"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestCatalogSeedIsIdempotent(t *testing.T) {
	ctx := context.Background()
	container, err := postgres.Run(ctx, "postgres:18.6-alpine",
		postgres.WithDatabase("tracker"), postgres.WithUsername("tracker"),
		postgres.WithPassword("integration-only"), postgres.BasicWaitStrategies())
	if err != nil {
		t.Fatalf("start PostgreSQL container: %v", err)
	}
	testcontainers.CleanupContainer(t, container)
	databaseURL, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}
	migrator, err := migrate.New("file://"+filepath.ToSlash(migrationsPath(t)), "pgx5"+databaseURL[len("postgres"):])
	if err != nil {
		t.Fatalf("create migrator: %v", err)
	}
	defer func() { _, _ = migrator.Close() }()
	if err := migrator.Up(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	pool, err := database.Open(ctx, databaseURL)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer pool.Close()
	seed, err := os.ReadFile(seedPath(t))
	if err != nil {
		t.Fatalf("read seed: %v", err)
	}
	for attempt := 0; attempt < 2; attempt++ {
		if _, err := pool.Exec(ctx, string(seed)); err != nil {
			t.Fatalf("apply seed attempt %d: %v", attempt+1, err)
		}
	}
	queries := catalogdb.New(pool)
	categories, err := queries.ListCategories(ctx)
	if err != nil {
		t.Fatalf("list categories: %v", err)
	}
	if len(categories) != 3 {
		t.Fatalf("category count = %d, want 3", len(categories))
	}
	topics, err := queries.CountTopics(ctx, catalogdb.CountTopicsParams{})
	if err != nil {
		t.Fatalf("count topics: %v", err)
	}
	if topics != 4 {
		t.Fatalf("topic count = %d, want 4", topics)
	}
}

func seedPath(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve seed test path")
	}
	return filepath.Join(filepath.Dir(filename), "..", "..", "seeds", "test_catalog.sql")
}

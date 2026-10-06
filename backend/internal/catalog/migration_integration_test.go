//go:build integration

package catalog_test

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestCatalogMigrationUpDownUp(t *testing.T) {
	ctx := context.Background()
	container, err := postgres.Run(ctx, "postgres:18.6-alpine",
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
	migrationsURL := "file://" + filepath.ToSlash(migrationsPath(t))
	migrator, err := migrate.New(migrationsURL, "pgx5"+databaseURL[len("postgres"):])
	if err != nil {
		t.Fatalf("create migrator: %v", err)
	}
	defer func() { _, _ = migrator.Close() }()

	if err := migrator.Up(); err != nil {
		t.Fatalf("migrate up: %v", err)
	}
	assertCatalogConstraints(t, ctx, databaseURL)
	if err := migrator.Down(); err != nil {
		t.Fatalf("migrate down: %v", err)
	}
	connection, err := pgx.Connect(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect after down: %v", err)
	}
	defer connection.Close(ctx)
	var table *string
	if err := connection.QueryRow(ctx, "SELECT to_regclass('public.topics')").Scan(&table); err != nil {
		t.Fatalf("check topics after down: %v", err)
	}
	if table != nil {
		t.Fatalf("topics table remains after down: %q", *table)
	}
	if err := migrator.Up(); err != nil {
		t.Fatalf("second migrate up: %v", err)
	}
}

func assertCatalogConstraints(t *testing.T, ctx context.Context, databaseURL string) {
	t.Helper()
	connection, err := pgx.Connect(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer connection.Close(ctx)

	var categoryID string
	err = connection.QueryRow(ctx, `INSERT INTO categories (slug, title, sort_order) VALUES ('go-core', 'Go Core', 0) RETURNING id`).Scan(&categoryID)
	if err != nil {
		t.Fatalf("insert category: %v", err)
	}
	_, err = connection.Exec(ctx, `INSERT INTO categories (slug, title, sort_order) VALUES ('go-core', 'Duplicate', 1)`)
	if err == nil {
		t.Fatal("duplicate category slug was accepted")
	}
	_, err = connection.Exec(ctx, `INSERT INTO topics (category_id, slug, title, summary, difficulty, sort_order, estimated_minutes) VALUES ($1, 'goroutines', 'Goroutines', '', 'expert', 0, 20)`, categoryID)
	if err == nil {
		t.Fatal("invalid difficulty was accepted")
	}
}

func migrationsPath(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve migration test path")
	}
	return filepath.Join(filepath.Dir(filename), "..", "..", "migrations")
}

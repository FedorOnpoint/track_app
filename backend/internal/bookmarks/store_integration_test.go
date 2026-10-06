//go:build integration

package bookmarks_test

import (
	"context"
	"errors"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/FedorOnpoint/track_app/backend/internal/bookmarks"
	"github.com/FedorOnpoint/track_app/backend/internal/platform/database"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestBookmarksAreIdempotentAndUserScoped(t *testing.T) {
	ctx := context.Background()
	container, err := postgres.Run(ctx, "postgres:18.6-alpine", postgres.WithDatabase("tracker"), postgres.WithUsername("tracker"), postgres.WithPassword("integration-only"), postgres.BasicWaitStrategies())
	if err != nil {
		t.Fatal(err)
	}
	testcontainers.CleanupContainer(t, container)
	databaseURL, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	migrator, err := migrate.New("file://"+filepath.ToSlash(migrationsPath(t)), "pgx5"+databaseURL[len("postgres"):])
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = migrator.Close() }()
	if err := migrator.Up(); err != nil {
		t.Fatal(err)
	}
	pool, err := database.Open(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	var firstUser, secondUser uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO users (login, password_hash) VALUES ('first', 'argon2id-test-hash-long-enough') RETURNING id`).Scan(&firstUser); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO users (login, password_hash) VALUES ('second', 'argon2id-test-hash-long-enough') RETURNING id`).Scan(&secondUser); err != nil {
		t.Fatal(err)
	}
	var categoryID uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO categories (slug, title, sort_order) VALUES ('go-core', 'Go Core', 0) RETURNING id`).Scan(&categoryID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO topics (category_id, slug, title, summary, difficulty, sort_order, estimated_minutes) VALUES ($1, 'goroutines', 'Goroutines', '', 'intermediate', 0, 20)`, categoryID); err != nil {
		t.Fatal(err)
	}
	store := bookmarks.NewPostgresStore(pool)
	for range 2 {
		if err := store.Set(ctx, firstUser, "goroutines", time.Now().UTC()); err != nil {
			t.Fatal(err)
		}
	}
	first, err := store.List(ctx, firstUser)
	if err != nil || len(first) != 1 {
		t.Fatalf("first list=%v err=%v", first, err)
	}
	second, err := store.List(ctx, secondUser)
	if err != nil || len(second) != 0 {
		t.Fatalf("second list=%v err=%v", second, err)
	}
	for range 2 {
		if err := store.Delete(ctx, firstUser, "goroutines"); err != nil {
			t.Fatal(err)
		}
	}
	after, err := store.List(ctx, firstUser)
	if err != nil || len(after) != 0 {
		t.Fatalf("after delete=%v err=%v", after, err)
	}
	if err := store.Set(ctx, firstUser, "missing", time.Now()); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("unknown topic error=%v", err)
	}
}

func migrationsPath(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve migration path")
	}
	return filepath.Join(filepath.Dir(filename), "..", "..", "migrations")
}

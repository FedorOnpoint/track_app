//go:build integration

package notes_test

import (
	"context"
	"errors"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/FedorOnpoint/track_app/backend/internal/notes"
	"github.com/FedorOnpoint/track_app/backend/internal/platform/database"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestNotesAreUserScoped(t *testing.T) {
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
	var owner, stranger, category uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO users (login, password_hash) VALUES ('owner', 'argon2id-test-hash-long-enough') RETURNING id`).Scan(&owner); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO users (login, password_hash) VALUES ('stranger', 'argon2id-test-hash-long-enough') RETURNING id`).Scan(&stranger); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO categories (slug, title, sort_order) VALUES ('go-core', 'Go Core', 0) RETURNING id`).Scan(&category); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO topics (category_id, slug, title, summary, difficulty, sort_order, estimated_minutes) VALUES ($1, 'goroutines', 'Goroutines', '', 'intermediate', 0, 20)`, category); err != nil {
		t.Fatal(err)
	}
	store := notes.NewPostgresStore(pool)
	now := time.Now().UTC()
	note, err := store.Create(ctx, owner, "goroutines", "owner text", now)
	if err != nil {
		t.Fatal(err)
	}
	ownerNotes, err := store.List(ctx, owner)
	if err != nil || len(ownerNotes) != 1 {
		t.Fatalf("owner notes=%v err=%v", ownerNotes, err)
	}
	strangerNotes, err := store.List(ctx, stranger)
	if err != nil || len(strangerNotes) != 0 {
		t.Fatalf("stranger notes=%v err=%v", strangerNotes, err)
	}
	if _, err := store.Update(ctx, stranger, note.ID, "stolen", now.Add(time.Second)); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("foreign update error=%v", err)
	}
	if err := store.Delete(ctx, stranger, note.ID); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("foreign delete error=%v", err)
	}
	updated, err := store.Update(ctx, owner, note.ID, "updated", now.Add(time.Second))
	if err != nil || updated.BodyMarkdown != "updated" {
		t.Fatalf("updated=%v err=%v", updated, err)
	}
	if err := store.Delete(ctx, owner, note.ID); err != nil {
		t.Fatal(err)
	}
	if err := store.Delete(ctx, owner, note.ID); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("missing delete error=%v", err)
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

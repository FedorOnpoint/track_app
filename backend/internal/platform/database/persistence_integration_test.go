//go:build integration

package database_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/FedorOnpoint/track_app/backend/internal/bookmarks"
	"github.com/FedorOnpoint/track_app/backend/internal/notes"
	"github.com/FedorOnpoint/track_app/backend/internal/platform/database"
	"github.com/FedorOnpoint/track_app/backend/internal/progress"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/google/uuid"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestUserDataPersistsAcrossApplicationPoolRestart(t *testing.T) {
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
	migrator, err := migrate.New("file://"+filepath.ToSlash(projectPath(t, "migrations")), "pgx5"+databaseURL[len("postgres"):])
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
	seed, err := os.ReadFile(projectPath(t, "seeds", "test_catalog.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, string(seed)); err != nil {
		t.Fatal(err)
	}
	var userID uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO users(login,password_hash) VALUES ('fedor','argon2id-test-hash-long-enough') RETURNING id`).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if _, err := progress.NewPostgresStore(pool).SetTopicProgress(ctx, userID, "goroutines", "learning", true, now); err != nil {
		t.Fatal(err)
	}
	if err := bookmarks.NewPostgresStore(pool).Set(ctx, userID, "goroutines", now); err != nil {
		t.Fatal(err)
	}
	if _, err := notes.NewPostgresStore(pool).Create(ctx, userID, "goroutines", "persistent note", now); err != nil {
		t.Fatal(err)
	}
	pool.Close()
	pool, err = database.Open(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	var status string
	if err := pool.QueryRow(ctx, `SELECT status FROM topic_progress WHERE user_id=$1`, userID).Scan(&status); err != nil || status != "learning" {
		t.Fatalf("status=%q err=%v", status, err)
	}
	bookmarked, err := bookmarks.NewPostgresStore(pool).List(ctx, userID)
	if err != nil || len(bookmarked) != 1 {
		t.Fatalf("bookmarks=%v err=%v", bookmarked, err)
	}
	savedNotes, err := notes.NewPostgresStore(pool).List(ctx, userID)
	if err != nil || len(savedNotes) != 1 || savedNotes[0].BodyMarkdown != "persistent note" {
		t.Fatalf("notes=%v err=%v", savedNotes, err)
	}
}

func projectPath(t *testing.T, parts ...string) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve project path")
	}
	base := filepath.Join(filepath.Dir(filename), "..", "..", "..")
	return filepath.Join(append([]string{base}, parts...)...)
}

//go:build integration

package dashboard_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/FedorOnpoint/track_app/backend/internal/dashboard"
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

func TestDashboardAggregatesAndRecentOrdering(t *testing.T) {
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
	seed, err := os.ReadFile(seedPath(t))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, string(seed)); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO categories (slug,title,sort_order) VALUES ('empty','Empty',99) ON CONFLICT DO NOTHING`); err != nil {
		t.Fatal(err)
	}
	var userID uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO users (login,password_hash) VALUES ('fedor','argon2id-test-hash-long-enough') RETURNING id`).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	var strangerID uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO users (login,password_hash) VALUES ('stranger','argon2id-test-hash-long-enough') RETURNING id`).Scan(&strangerID); err != nil {
		t.Fatal(err)
	}
	progressStore := progress.NewPostgresStore(pool)
	stamp := time.Date(2026, 10, 7, 10, 0, 0, 0, time.UTC)
	if _, err := progressStore.SetTopicProgress(ctx, userID, "interfaces", "mastered", false, stamp); err != nil {
		t.Fatal(err)
	}
	if _, err := progressStore.SetTopicProgress(ctx, userID, "context", "review", true, stamp); err != nil {
		t.Fatal(err)
	}
	if err := progressStore.RecordTopicVisit(ctx, userID, "interfaces", stamp.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := progressStore.RecordTopicVisit(ctx, userID, "context", stamp.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := notes.NewPostgresStore(pool).Create(ctx, userID, "context", "recent note", stamp.Add(3*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := progressStore.SetTopicProgress(ctx, strangerID, "goroutines", "review", true, stamp.Add(10*time.Minute)); err != nil {
		t.Fatal(err)
	}
	result, err := dashboard.NewPostgresStore(pool).Get(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if result.Overall.Total != 4 || result.Overall.Mastered != 1 {
		t.Fatalf("overall=%+v", result.Overall)
	}
	if result.ContinueTopic == nil || result.ContinueTopic.Slug != "context" {
		t.Fatalf("continue=%+v", result.ContinueTopic)
	}
	if len(result.ReviewTopics) != 1 || result.ReviewTopics[0].Slug != "context" {
		t.Fatalf("review=%+v", result.ReviewTopics)
	}
	if len(result.RecentTopics) < 2 || result.RecentTopics[0].Slug != "context" || result.RecentTopics[1].Slug != "interfaces" {
		t.Fatalf("recent=%+v", result.RecentTopics)
	}
	if len(result.RecentNotes) != 1 || result.RecentNotes[0].BodyMarkdown != "recent note" {
		t.Fatalf("notes=%+v", result.RecentNotes)
	}
	foundEmpty := false
	for _, category := range result.Categories {
		if category.Slug == "empty" {
			foundEmpty = true
			if category.Total != 0 || category.Mastered != 0 {
				t.Fatalf("empty=%+v", category)
			}
		}
	}
	if !foundEmpty {
		t.Fatal("empty category missing")
	}
}

func migrationsPath(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve path")
	}
	return filepath.Join(filepath.Dir(filename), "..", "..", "migrations")
}
func seedPath(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve path")
	}
	return filepath.Join(filepath.Dir(filename), "..", "..", "seeds", "test_catalog.sql")
}

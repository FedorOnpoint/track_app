//go:build integration

package catalog_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/FedorOnpoint/track_app/backend/internal/bookmarks"
	"github.com/FedorOnpoint/track_app/backend/internal/catalog"
	"github.com/FedorOnpoint/track_app/backend/internal/platform/database"
	"github.com/FedorOnpoint/track_app/backend/internal/progress"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/google/uuid"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestPersonalizedTopicFiltersAndSearch(t *testing.T) {
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
	var userID, strangerID uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO users(login,password_hash) VALUES ('owner','argon2id-test-hash-long-enough') RETURNING id`).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO users(login,password_hash) VALUES ('stranger','argon2id-test-hash-long-enough') RETURNING id`).Scan(&strangerID); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if _, err := progress.NewPostgresStore(pool).SetTopicProgress(ctx, userID, "goroutines", "review", true, now); err != nil {
		t.Fatal(err)
	}
	if err := bookmarks.NewPostgresStore(pool).Set(ctx, userID, "interfaces", now); err != nil {
		t.Fatal(err)
	}
	if err := bookmarks.NewPostgresStore(pool).Set(ctx, strangerID, "context", now); err != nil {
		t.Fatal(err)
	}
	store := catalog.NewPostgresPersonalizedStore(pool)
	truth := true
	bookmarked, err := store.ListUserTopics(ctx, catalog.UserTopicParams{UserID: userID, Bookmarked: &truth, Sort: "title", PageSize: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(bookmarked) != 1 || bookmarked[0].Slug != "interfaces" {
		t.Fatalf("bookmarked=%+v", bookmarked)
	}
	review, err := store.ListUserTopics(ctx, catalog.UserTopicParams{UserID: userID, ReviewRequired: &truth, Sort: "title", PageSize: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(review) != 1 || review[0].Slug != "goroutines" {
		t.Fatalf("review=%+v", review)
	}
	query := "safe point"
	searched, err := store.ListUserTopics(ctx, catalog.UserTopicParams{UserID: userID, Query: &query, Sort: "title", PageSize: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(searched) != 1 || searched[0].Slug != "goroutines" {
		t.Fatalf("searched=%+v", searched)
	}
	status := "not_started"
	total, err := store.CountUserTopics(ctx, catalog.UserTopicParams{UserID: userID, Status: &status})
	if err != nil {
		t.Fatal(err)
	}
	if total != 3 {
		t.Fatalf("not_started total=%d", total)
	}
	firstPage, err := store.ListUserTopics(ctx, catalog.UserTopicParams{UserID: userID, Sort: "title", PageSize: 1})
	if err != nil {
		t.Fatal(err)
	}
	secondPage, err := store.ListUserTopics(ctx, catalog.UserTopicParams{UserID: userID, Sort: "title", PageSize: 1, PageOffset: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(firstPage) != 1 || len(secondPage) != 1 || firstPage[0].Slug == secondPage[0].Slug {
		t.Fatalf("pagination first=%+v second=%+v", firstPage, secondPage)
	}
}

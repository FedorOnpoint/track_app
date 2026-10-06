//go:build integration

package progress_test

import (
	"context"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/FedorOnpoint/track_app/backend/internal/platform/database"
	"github.com/FedorOnpoint/track_app/backend/internal/progress"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestProgressMigrationEnforcesStateConstraints(t *testing.T) {
	ctx := context.Background()
	container, err := postgres.Run(ctx, "postgres:18.6-alpine", postgres.WithDatabase("tracker"), postgres.WithUsername("tracker"), postgres.WithPassword("integration-only"), postgres.BasicWaitStrategies())
	if err != nil {
		t.Fatalf("start PostgreSQL: %v", err)
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
	connection, err := pgx.Connect(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close(ctx)
	var userID, topicID, itemID string
	if err := connection.QueryRow(ctx, `INSERT INTO users (login, password_hash) VALUES ('fedor', 'argon2id-test-hash-long-enough') RETURNING id`).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	var categoryID string
	if err := connection.QueryRow(ctx, `INSERT INTO categories (slug, title, sort_order) VALUES ('go-core', 'Go Core', 0) RETURNING id`).Scan(&categoryID); err != nil {
		t.Fatal(err)
	}
	if err := connection.QueryRow(ctx, `INSERT INTO topics (category_id, slug, title, summary, difficulty, sort_order, estimated_minutes) VALUES ($1, 'goroutines', 'Goroutines', '', 'intermediate', 0, 20) RETURNING id`, categoryID).Scan(&topicID); err != nil {
		t.Fatal(err)
	}
	if err := connection.QueryRow(ctx, `INSERT INTO checklist_items (topic_id, stable_key, text, sort_order) VALUES ($1, 'explain', 'Explain goroutines', 0) RETURNING id`, topicID).Scan(&itemID); err != nil {
		t.Fatal(err)
	}
	if _, err := connection.Exec(ctx, `INSERT INTO topic_progress (user_id, topic_id, status) VALUES ($1, $2, 'unknown')`, userID, topicID); err == nil {
		t.Fatal("invalid progress status was accepted")
	}
	if _, err := connection.Exec(ctx, `INSERT INTO topic_progress (user_id, topic_id, status) VALUES ($1, $2, 'learning')`, userID, topicID); err != nil {
		t.Fatalf("insert valid topic progress: %v", err)
	}
	if _, err := connection.Exec(ctx, `INSERT INTO topic_progress (user_id, topic_id, status) VALUES ($1, $2, 'review')`, userID, topicID); err == nil {
		t.Fatal("duplicate user/topic progress was accepted")
	}
	if _, err := connection.Exec(ctx, `INSERT INTO checklist_progress (user_id, checklist_item_id, completed) VALUES ($1, $2, true)`, userID, itemID); err != nil {
		t.Fatalf("insert valid checklist progress: %v", err)
	}
	if _, err := connection.Exec(ctx, `INSERT INTO checklist_progress (user_id, checklist_item_id, completed) VALUES ($1, $2, false)`, userID, itemID); err == nil {
		t.Fatal("duplicate user/checklist progress was accepted")
	}

	if _, err := connection.Exec(ctx, `DELETE FROM checklist_progress; DELETE FROM topic_progress`); err != nil {
		t.Fatal(err)
	}
	pool, err := database.Open(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	store := progress.NewPostgresStore(pool)
	parsedUserID := uuid.MustParse(userID)
	statuses := []string{"not_started", "learning", "review", "mastered"}
	var wait sync.WaitGroup
	errorsFound := make(chan error, 24)
	for index := 0; index < 24; index++ {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			_, err := store.SetTopicProgress(ctx, parsedUserID, "goroutines", statuses[index%len(statuses)], index%2 == 0, time.Now().UTC())
			errorsFound <- err
		}(index)
	}
	wait.Wait()
	close(errorsFound)
	for err := range errorsFound {
		if err != nil {
			t.Fatalf("concurrent progress update: %v", err)
		}
	}
	var count int
	var finalStatus string
	if err := connection.QueryRow(ctx, `SELECT count(*), min(status) FROM topic_progress WHERE user_id = $1 AND topic_id = $2`, userID, topicID).Scan(&count, &finalStatus); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("progress rows=%d, want 1", count)
	}
	valid := false
	for _, status := range statuses {
		valid = valid || finalStatus == status
	}
	if !valid {
		t.Fatalf("final status=%q is invalid", finalStatus)
	}
	parsedItemID := uuid.MustParse(itemID)
	for range 2 {
		if _, err := store.SetChecklistProgress(ctx, parsedUserID, "goroutines", parsedItemID, true, time.Now().UTC()); err != nil {
			t.Fatalf("idempotent checklist update: %v", err)
		}
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

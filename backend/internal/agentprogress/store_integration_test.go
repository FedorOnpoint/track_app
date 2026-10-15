//go:build integration

package agentprogress

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	dsn := os.Getenv("AGENT_TEST_DATABASE_URL")
	if dsn == "" {
		container, err := postgres.Run(ctx, "postgres:18.6-alpine", postgres.WithDatabase("tracker"), postgres.WithUsername("tracker"), postgres.WithPassword("integration-only"), postgres.BasicWaitStrategies())
		if err != nil {
			t.Fatal(err)
		}
		testcontainers.CleanupContainer(t, container)
		var errURL error
		dsn, errURL = container.ConnectionString(ctx, "sslmode=disable")
		if errURL != nil {
			t.Fatal(errURL)
		}
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ConnConfig.Host != "localhost" && cfg.ConnConfig.Host != "127.0.0.1" && cfg.ConnConfig.Host != "::1" {
		t.Fatal("integration database must be local")
	}
	admin, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	schema := "agent_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = admin.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE"); admin.Close() })
	cfg.ConnConfig.RuntimeParams["search_path"] = schema + ",public"
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	files, err := filepath.Glob("../../migrations/*.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range files {
		sql, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, string(sql)); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
	}
	return pool
}

func TestStoreAtomicScopedAndIdempotent(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	store := NewPostgresStore(pool)
	owner, other, category, topic, item := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`INSERT INTO users(id,login,password_hash) VALUES($1,'owner','argon2id-test-hash-long-enough'),($2,'other','argon2id-test-hash-long-enough')`, owner, other)
	exec(`INSERT INTO categories(id,slug,title,sort_order) VALUES($1,'go','Go',0)`, category)
	exec(`INSERT INTO topics(id,category_id,slug,title,summary,difficulty,sort_order,estimated_minutes) VALUES($1,$2,'go-maps','Maps','','basic',0,10)`, topic, category)
	exec(`INSERT INTO checklist_items(id,topic_id,stable_key,text,sort_order) VALUES($1,$2,'nil-map','Nil map',0)`, item, topic)
	yes, no := true, false
	summary := "Learned maps"
	input := Update{Status: "mastered", ReviewRequired: &yes, Checklist: map[string]*bool{"nil-map": &yes}, Summary: &summary}
	first, err := store.Apply(ctx, owner, "go-maps", "session-1", input)
	if err != nil {
		t.Fatal(err)
	}
	if first.NoteID == nil || first.Status != "mastered" {
		t.Fatalf("result=%+v", first)
	}
	replay, err := store.Apply(ctx, owner, "go-maps", "session-1", input)
	if err != nil || replay.NoteID == nil || *replay.NoteID != *first.NoteID {
		t.Fatalf("replay=%+v err=%v", replay, err)
	}
	changed := input
	changed.Status = "learning"
	if _, err := store.Apply(ctx, owner, "go-maps", "session-1", changed); !errors.Is(err, ErrConflict) {
		t.Fatalf("conflict=%v", err)
	}
	preserved, err := store.Apply(ctx, owner, "go-maps", "session-2", Update{Status: "learning"})
	if err != nil || preserved.Status != "mastered" || !preserved.ReviewRequired {
		t.Fatalf("preserved=%+v err=%v", preserved, err)
	}
	// A bad checklist key must roll back every change and must not claim the update ID.
	bad := Update{ReviewRequired: &no, Checklist: map[string]*bool{"unknown": &yes}, Summary: &summary}
	if _, err := store.Apply(ctx, owner, "go-maps", "session-bad", bad); !errors.Is(err, ErrNotFound) {
		t.Fatalf("bad item=%v", err)
	}
	topics, err := store.Topics(ctx, owner)
	if err != nil || len(topics) != 1 {
		t.Fatalf("topics=%v err=%v", topics, err)
	}
	if topics[0].Status != "mastered" || !topics[0].ReviewRequired || !topics[0].Checklist[0].Completed {
		t.Fatalf("rollback failed: %+v", topics)
	}
	var notes, updates int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM notes WHERE user_id=$1`, owner).Scan(&notes); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM agent_progress_updates WHERE user_id=$1`, owner).Scan(&updates); err != nil {
		t.Fatal(err)
	}
	if notes != 1 || updates != 2 {
		t.Fatalf("notes=%d updates=%d", notes, updates)
	}
	// Retry a corrected request with the rolled-back key.
	if _, err := store.Apply(ctx, owner, "go-maps", "session-bad", Update{Checklist: map[string]*bool{"nil-map": &no}}); err != nil {
		t.Fatal(err)
	}
	otherTopics, err := store.Topics(ctx, other)
	if err != nil || otherTopics[0].Status != "not_started" || otherTopics[0].Checklist[0].Completed {
		t.Fatalf("user isolation: %v %v", otherTopics, err)
	}
	if _, err := store.Apply(ctx, owner, "missing", "x", input); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing=%v", err)
	}
	if _, err := store.Apply(ctx, uuid.New(), "go-maps", "x", input); !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("missing owner=%v", err)
	}
	// Concurrent identical submissions create exactly one note.
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := store.Apply(ctx, other, "go-maps", "concurrent", input)
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM notes WHERE user_id=$1`, other).Scan(&notes); err != nil || notes != 1 {
		t.Fatalf("concurrent notes=%d err=%v", notes, err)
	}
	// Migration down/up leaves user notes intact and recreates the ledger.
	for _, path := range []string{"../../migrations/000008_create_agent_updates.down.sql", "../../migrations/000008_create_agent_updates.up.sql"} {
		sql, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		exec(string(sql))
	}
}

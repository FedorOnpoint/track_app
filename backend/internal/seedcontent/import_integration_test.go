//go:build integration

package seedcontent

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/FedorOnpoint/track_app/backend/internal/platform/database"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestImportPreservesUserDataAndStableIDs(t *testing.T) {
	pool := seedDatabase(t)
	ctx := context.Background()
	doc := validDocument()
	if err := Import(ctx, pool, []Document{doc}); err != nil {
		t.Fatal(err)
	}
	var topicID, checklistID, userID uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT id FROM topics WHERE slug='interfaces'`).Scan(&topicID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT id FROM checklist_items WHERE topic_id=$1 AND stable_key='one'`, topicID).Scan(&checklistID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO users(login,password_hash) VALUES ('seed-test','argon2id-test-hash-long-enough') RETURNING id`).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{
		`INSERT INTO topic_progress(user_id,topic_id,status,review_required) VALUES ($1,$2,'learning',true)`,
		`INSERT INTO bookmarks(user_id,topic_id) VALUES ($1,$2)`,
		`INSERT INTO notes(user_id,topic_id,body_markdown) VALUES ($1,$2,'keep this note')`,
	} {
		if _, err := pool.Exec(ctx, query, userID, topicID); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := pool.Exec(ctx, `INSERT INTO checklist_progress(user_id,checklist_item_id,completed) VALUES ($1,$2,true)`, userID, checklistID); err != nil {
		t.Fatal(err)
	}
	doc.Version = 2
	doc.Topics[0].Summary = "Обновлённая теория"
	doc.Topics[0].Checklist[0].Text = "Обновлённый пункт"
	doc.Topics[0].Checklist[0].SortOrder, doc.Topics[0].Checklist[1].SortOrder = 2, 1
	doc.Topics[0].Questions[0].SortOrder, doc.Topics[0].Questions[1].SortOrder = 2, 1
	for i := 0; i < 2; i++ {
		if err := Import(ctx, pool, []Document{doc}); err != nil {
			t.Fatal(err)
		}
	}
	var actualTopic, actualChecklist uuid.UUID
	var summary, status, body, itemText string
	var bookmarked, completed, review bool
	var version, count, order int
	if err := pool.QueryRow(ctx, `SELECT t.id,t.summary,t.content_version,p.status,p.review_required,n.body_markdown,EXISTS(SELECT 1 FROM bookmarks b WHERE b.user_id=$1 AND b.topic_id=t.id) FROM topics t JOIN topic_progress p ON p.topic_id=t.id AND p.user_id=$1 JOIN notes n ON n.topic_id=t.id AND n.user_id=$1 WHERE t.slug='interfaces'`, userID).Scan(&actualTopic, &summary, &version, &status, &review, &body, &bookmarked); err != nil {
		t.Fatal(err)
	}
	if actualTopic != topicID || summary != "Обновлённая теория" || version != 2 || status != "learning" || !review || !bookmarked || body != "keep this note" {
		t.Fatalf("user state changed: topic=%v summary=%q version=%d status=%q review=%v bookmark=%v note=%q", actualTopic, summary, version, status, review, bookmarked, body)
	}
	if err := pool.QueryRow(ctx, `SELECT c.id,c.text,c.sort_order,p.completed FROM checklist_items c JOIN checklist_progress p ON p.checklist_item_id=c.id WHERE c.topic_id=$1 AND c.stable_key='one'`, topicID).Scan(&actualChecklist, &itemText, &order, &completed); err != nil {
		t.Fatal(err)
	}
	if actualChecklist != checklistID || !completed || itemText != "Обновлённый пункт" || order != 2 {
		t.Fatalf("checklist changed: id=%v text=%q order=%d completed=%v", actualChecklist, itemText, order, completed)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM checklist_items WHERE topic_id=$1`, topicID).Scan(&count); err != nil || count != 3 {
		t.Fatalf("count=%d err=%v", count, err)
	}
	if err := Import(ctx, pool, []Document{validDocument()}); err == nil {
		t.Fatal("accepted older content version")
	}
}

func TestImportValidatesWholeBatchAndRollsBackDatabaseFailure(t *testing.T) {
	pool := seedDatabase(t)
	ctx := context.Background()
	valid, invalid := validDocument(), validDocument()
	invalid.Category.Slug = "invalid-category"
	invalid.Topics[0].Summary = ""
	if err := Import(ctx, pool, []Document{valid, invalid}); err == nil {
		t.Fatal("accepted invalid batch")
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM categories`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("partial validation write: count=%d err=%v", count, err)
	}
	if _, err := pool.Exec(ctx, `CREATE FUNCTION reject_resources() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'resource write rejected'; END $$; CREATE TRIGGER reject_resources BEFORE INSERT ON resources FOR EACH ROW EXECUTE FUNCTION reject_resources()`); err != nil {
		t.Fatal(err)
	}
	if err := Import(ctx, pool, []Document{valid}); err == nil {
		t.Fatal("ignored database failure")
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM topics`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("partial transaction: count=%d err=%v", count, err)
	}
}

func TestImportRejectsStaleCategoryWithDifferentTopicSlugs(t *testing.T) {
	pool := seedDatabase(t)
	ctx := context.Background()
	current := validDocument()
	current.Version = 2
	if err := Import(ctx, pool, []Document{current}); err != nil {
		t.Fatal(err)
	}
	stale := validDocument()
	stale.Topics[0].Slug = "legacy"
	stale.Category.Title = "Stale title"
	if err := Import(ctx, pool, []Document{stale}); err == nil {
		t.Fatal("accepted stale category with different topic slug")
	}
	var title string
	if err := pool.QueryRow(ctx, `SELECT title FROM categories WHERE slug='go-core'`).Scan(&title); err != nil || title != "Go Core" {
		t.Fatalf("title=%q err=%v", title, err)
	}
}

func TestImportReconcilesUnkeyedCollectionsWithoutDuplicates(t *testing.T) {
	pool := seedDatabase(t)
	ctx := context.Background()
	doc := validDocument()
	doc.Topics[0].Practice = []Practice{{Title: "Practice", Description: "Implement", ExpectedOutcome: "Tests pass", SortOrder: 1}}
	if err := Import(ctx, pool, []Document{doc}); err != nil {
		t.Fatal(err)
	}
	doc.Version = 2
	doc.Topics[0].Sections[0].SortOrder = 2
	doc.Topics[0].Resources[0].SortOrder = 2
	doc.Topics[0].Questions[0].FollowUps = nil
	doc.Topics[0].Practice = nil
	if err := Import(ctx, pool, []Document{doc}); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		table string
		want  int
	}{{"resources", 1}, {"topic_sections", 2}, {"follow_up_questions", 0}, {"practice_recommendations", 0}} {
		var count int
		if err := pool.QueryRow(ctx, "SELECT count(*) FROM "+tc.table).Scan(&count); err != nil || count != tc.want {
			t.Fatalf("table=%s count=%d want=%d err=%v", tc.table, count, tc.want, err)
		}
	}
}

func TestImportRetainsCategoryVersionAfterMovingLastTopic(t *testing.T) {
	pool := seedDatabase(t)
	ctx := context.Background()
	original := validDocument()
	original.Version = 2
	if err := Import(ctx, pool, []Document{original}); err != nil {
		t.Fatal(err)
	}
	moved := validDocument()
	moved.Version = 3
	moved.Category.Slug = "other-category"
	if err := Import(ctx, pool, []Document{moved}); err != nil {
		t.Fatal(err)
	}
	stale := validDocument()
	stale.Topics[0].Slug = "legacy"
	if err := Import(ctx, pool, []Document{stale}); err == nil {
		t.Fatal("lost category version after moving its last topic")
	}
}

func TestSeedVersionMigrationBackfillsExistingContentAndIsReversible(t *testing.T) {
	pool := seedDatabase(t)
	ctx := context.Background()
	_, file, _, _ := runtime.Caller(0)
	base := filepath.Join(filepath.Dir(file), "..", "..", "migrations")
	apply := func(name string) {
		t.Helper()
		sql, err := os.ReadFile(filepath.Join(base, name))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, string(sql)); err != nil {
			t.Fatal(err)
		}
	}
	apply("000006_create_catalog_seed_versions.down.sql")
	var categoryID uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO categories(slug,title,sort_order) VALUES('existing','Existing',10) RETURNING id`).Scan(&categoryID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO topics(category_id,slug,title,summary,difficulty,sort_order,estimated_minutes,content_version) VALUES($1,'existing-topic','Existing','summary','basic',10,20,5)`, categoryID); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		apply("000006_create_catalog_seed_versions.up.sql")
		var version int
		if err := pool.QueryRow(ctx, `SELECT content_version FROM catalog_seed_versions WHERE category_id=$1`, categoryID).Scan(&version); err != nil || version != 5 {
			t.Fatalf("version=%d err=%v", version, err)
		}
		if i == 0 {
			apply("000006_create_catalog_seed_versions.down.sql")
		}
	}
}

func seedDatabase(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	container, err := postgres.Run(ctx, "postgres:18.6-alpine", postgres.WithDatabase("tracker"), postgres.WithUsername("tracker"), postgres.WithPassword("integration-only"), postgres.BasicWaitStrategies())
	if err != nil {
		t.Fatal(err)
	}
	testcontainers.CleanupContainer(t, container)
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	_, file, _, _ := runtime.Caller(0)
	base := filepath.Join(filepath.Dir(file), "..", "..")
	migrator, err := migrate.New("file://"+filepath.ToSlash(filepath.Join(base, "migrations")), "pgx5"+dsn[len("postgres"):])
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = migrator.Close() }()
	if err := migrator.Up(); err != nil {
		t.Fatal(err)
	}
	pool, err := database.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func TestGoCoreSeedImportsCompleteTopicContent(t *testing.T) {
	pool := seedDatabase(t)
	_, file, _, _ := runtime.Caller(0)
	seed, err := os.Open(filepath.Join(filepath.Dir(file), "..", "..", "seeds", "go-core.v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer seed.Close()
	doc, err := Decode(seed)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := Import(context.Background(), pool, []Document{doc}); err != nil {
			t.Fatal(err)
		}
	}
	var count int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM topics t JOIN categories c ON c.id=t.category_id WHERE c.slug='go-core'`).Scan(&count); err != nil || count != 10 {
		t.Fatalf("count=%d err=%v", count, err)
	}
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM topics t WHERE NOT EXISTS(SELECT 1 FROM topic_sections s WHERE s.topic_id=t.id AND s.sort_order=0) OR NOT EXISTS(SELECT 1 FROM resources r WHERE r.topic_id=t.id) OR (SELECT count(*) FROM interview_questions q WHERE q.topic_id=t.id)<3`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("incomplete topics=%d err=%v", count, err)
	}
}

//go:build integration

package seedcontent

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestFullCatalogReseedPreservesEveryTopicUserState(t *testing.T) {
	documents := productionDocuments(t)
	if err := ValidateCatalog(documents, true); err != nil {
		t.Fatal(err)
	}
	pool := seedDatabase(t)
	ctx := context.Background()
	if err := Import(ctx, pool, documents); err != nil {
		t.Fatal(err)
	}
	var userID uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO users(login,password_hash) VALUES('full-seed','hash-long-enough-for-test-only') RETURNING id`).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		`INSERT INTO topic_progress(user_id,topic_id,status,review_required) SELECT $1,id,'learning',true FROM topics`,
		`INSERT INTO bookmarks(user_id,topic_id) SELECT $1,id FROM topics`,
		`INSERT INTO notes(user_id,topic_id,body_markdown) SELECT $1,id,'preserve:'||slug FROM topics`,
		`INSERT INTO checklist_progress(user_id,checklist_item_id,completed) SELECT $1,id,true FROM checklist_items`,
	} {
		if _, err := pool.Exec(ctx, q, userID); err != nil {
			t.Fatal(err)
		}
	}
	query := `SELECT md5(string_agg(t.id::text||':'||t.slug||':'||p.status||':'||p.review_required::text||':'||n.body_markdown||':'||b.topic_id::text,'|' ORDER BY t.slug)) FROM topics t JOIN topic_progress p ON p.topic_id=t.id AND p.user_id=$1 JOIN notes n ON n.topic_id=t.id AND n.user_id=$1 JOIN bookmarks b ON b.topic_id=t.id AND b.user_id=$1`
	var before, after string
	if err := pool.QueryRow(ctx, query, userID).Scan(&before); err != nil {
		t.Fatal(err)
	}
	checklistQuery := `SELECT md5(string_agg(c.id::text||':'||p.completed::text,'|' ORDER BY c.id)) FROM checklist_items c JOIN checklist_progress p ON p.checklist_item_id=c.id AND p.user_id=$1`
	var checklistBefore, checklistAfter string
	if err := pool.QueryRow(ctx, checklistQuery, userID).Scan(&checklistBefore); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := Import(ctx, pool, documents); err != nil {
			t.Fatal(err)
		}
	}
	if err := pool.QueryRow(ctx, query, userID).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Fatal("personal state or IDs changed on reseed")
	}
	if err := pool.QueryRow(ctx, checklistQuery, userID).Scan(&checklistAfter); err != nil {
		t.Fatal(err)
	}
	if checklistBefore != checklistAfter {
		t.Fatal("checklist state or IDs changed")
	}
	var categories, topics int
	if err := pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM categories),(SELECT count(*) FROM topics)`).Scan(&categories, &topics); err != nil || categories != 13 || topics < 80 || topics > 120 {
		t.Fatalf("categories=%d topics=%d err=%v", categories, topics, err)
	}
}

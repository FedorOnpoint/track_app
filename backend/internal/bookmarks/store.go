package bookmarks

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Bookmark struct {
	TopicID          uuid.UUID
	Slug             string
	Title            string
	Summary          string
	Difficulty       string
	EstimatedMinutes int32
	CategorySlug     string
	CategoryTitle    string
	CreatedAt        time.Time
}

type Store interface {
	List(context.Context, uuid.UUID) ([]Bookmark, error)
	Set(context.Context, uuid.UUID, string, time.Time) error
	Delete(context.Context, uuid.UUID, string) error
}

type database interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

type PostgresStore struct{ db database }

func NewPostgresStore(db database) *PostgresStore { return &PostgresStore{db: db} }

func (store *PostgresStore) List(ctx context.Context, userID uuid.UUID) ([]Bookmark, error) {
	rows, err := store.db.Query(ctx, `
SELECT t.id, t.slug, t.title, t.summary, t.difficulty, t.estimated_minutes, c.slug, c.title, b.created_at
FROM bookmarks b JOIN topics t ON t.id = b.topic_id JOIN categories c ON c.id = t.category_id
WHERE b.user_id = $1 ORDER BY b.created_at DESC, t.slug`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]Bookmark, 0)
	for rows.Next() {
		var item Bookmark
		if err := rows.Scan(&item.TopicID, &item.Slug, &item.Title, &item.Summary, &item.Difficulty, &item.EstimatedMinutes, &item.CategorySlug, &item.CategoryTitle, &item.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (store *PostgresStore) Set(ctx context.Context, userID uuid.UUID, slug string, now time.Time) error {
	var topicID uuid.UUID
	return store.db.QueryRow(ctx, `
WITH topic AS (SELECT id FROM topics WHERE slug = $2), inserted AS (
  INSERT INTO bookmarks (user_id, topic_id, created_at) SELECT $1, id, $3 FROM topic
  ON CONFLICT (user_id, topic_id) DO NOTHING RETURNING topic_id
)
SELECT id FROM topic`, userID, slug, now).Scan(&topicID)
}

func (store *PostgresStore) Delete(ctx context.Context, userID uuid.UUID, slug string) error {
	var topicID uuid.UUID
	return store.db.QueryRow(ctx, `
WITH topic AS (SELECT id FROM topics WHERE slug = $2), deleted AS (
  DELETE FROM bookmarks USING topic WHERE bookmarks.user_id = $1 AND bookmarks.topic_id = topic.id RETURNING bookmarks.topic_id
)
SELECT id FROM topic`, userID, slug).Scan(&topicID)
}

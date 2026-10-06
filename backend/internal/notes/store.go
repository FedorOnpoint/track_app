package notes

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Note struct {
	ID           uuid.UUID
	TopicID      uuid.UUID
	TopicSlug    string
	TopicTitle   string
	BodyMarkdown string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type Store interface {
	List(context.Context, uuid.UUID) ([]Note, error)
	Create(context.Context, uuid.UUID, string, string, time.Time) (Note, error)
	Update(context.Context, uuid.UUID, uuid.UUID, string, time.Time) (Note, error)
	Delete(context.Context, uuid.UUID, uuid.UUID) error
}

type database interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

type PostgresStore struct{ db database }

func NewPostgresStore(db database) *PostgresStore { return &PostgresStore{db: db} }

func (store *PostgresStore) List(ctx context.Context, userID uuid.UUID) ([]Note, error) {
	rows, err := store.db.Query(ctx, `
SELECT n.id, n.topic_id, t.slug, t.title, n.body_markdown, n.created_at, n.updated_at
FROM notes n JOIN topics t ON t.id = n.topic_id
WHERE n.user_id = $1 ORDER BY n.updated_at DESC, n.id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]Note, 0)
	for rows.Next() {
		var note Note
		if err := rows.Scan(&note.ID, &note.TopicID, &note.TopicSlug, &note.TopicTitle, &note.BodyMarkdown, &note.CreatedAt, &note.UpdatedAt); err != nil {
			return nil, err
		}
		result = append(result, note)
	}
	return result, rows.Err()
}

func (store *PostgresStore) Create(ctx context.Context, userID uuid.UUID, slug, body string, now time.Time) (Note, error) {
	var note Note
	err := store.db.QueryRow(ctx, `
WITH inserted AS (
  INSERT INTO notes (user_id, topic_id, body_markdown, created_at, updated_at)
  SELECT $1, id, $3, $4, $4 FROM topics WHERE slug = $2
  RETURNING id, topic_id, body_markdown, created_at, updated_at
)
SELECT i.id, i.topic_id, t.slug, t.title, i.body_markdown, i.created_at, i.updated_at
FROM inserted i JOIN topics t ON t.id = i.topic_id`, userID, slug, body, now).Scan(&note.ID, &note.TopicID, &note.TopicSlug, &note.TopicTitle, &note.BodyMarkdown, &note.CreatedAt, &note.UpdatedAt)
	return note, err
}

func (store *PostgresStore) Update(ctx context.Context, userID, noteID uuid.UUID, body string, now time.Time) (Note, error) {
	var note Note
	err := store.db.QueryRow(ctx, `
WITH updated AS (
  UPDATE notes SET body_markdown = $3, updated_at = $4 WHERE id = $2 AND user_id = $1
  RETURNING id, topic_id, body_markdown, created_at, updated_at
)
SELECT u.id, u.topic_id, t.slug, t.title, u.body_markdown, u.created_at, u.updated_at
FROM updated u JOIN topics t ON t.id = u.topic_id`, userID, noteID, body, now).Scan(&note.ID, &note.TopicID, &note.TopicSlug, &note.TopicTitle, &note.BodyMarkdown, &note.CreatedAt, &note.UpdatedAt)
	return note, err
}

func (store *PostgresStore) Delete(ctx context.Context, userID, noteID uuid.UUID) error {
	var deleted uuid.UUID
	return store.db.QueryRow(ctx, `DELETE FROM notes WHERE id = $1 AND user_id = $2 RETURNING id`, noteID, userID).Scan(&deleted)
}

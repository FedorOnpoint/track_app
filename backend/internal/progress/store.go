package progress

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type TopicProgress struct {
	TopicID        uuid.UUID
	Status         string
	ReviewRequired bool
	LastOpenedAt   *time.Time
	UpdatedAt      time.Time
}

type ChecklistProgress struct {
	ChecklistItemID uuid.UUID
	Completed       bool
	UpdatedAt       time.Time
}

type Store interface {
	SetTopicProgress(context.Context, uuid.UUID, string, string, bool, time.Time) (TopicProgress, error)
	SetChecklistProgress(context.Context, uuid.UUID, string, uuid.UUID, bool, time.Time) (ChecklistProgress, error)
}

type queryRower interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

type PostgresStore struct{ db queryRower }

func NewPostgresStore(db queryRower) *PostgresStore { return &PostgresStore{db: db} }

func (store *PostgresStore) SetTopicProgress(ctx context.Context, userID uuid.UUID, slug, status string, reviewRequired bool, now time.Time) (TopicProgress, error) {
	row := store.db.QueryRow(ctx, `
INSERT INTO topic_progress (user_id, topic_id, status, review_required, updated_at)
SELECT $1, topics.id, $3, $4, $5 FROM topics WHERE topics.slug = $2
ON CONFLICT (user_id, topic_id) DO UPDATE
SET status = EXCLUDED.status, review_required = EXCLUDED.review_required, updated_at = EXCLUDED.updated_at
RETURNING topic_id, status, review_required, last_opened_at, updated_at`, userID, slug, status, reviewRequired, now)
	var result TopicProgress
	err := row.Scan(&result.TopicID, &result.Status, &result.ReviewRequired, &result.LastOpenedAt, &result.UpdatedAt)
	return result, err
}

func (store *PostgresStore) SetChecklistProgress(ctx context.Context, userID uuid.UUID, slug string, itemID uuid.UUID, completed bool, now time.Time) (ChecklistProgress, error) {
	row := store.db.QueryRow(ctx, `
INSERT INTO checklist_progress (user_id, checklist_item_id, completed, updated_at)
SELECT $1, items.id, $4, $5
FROM checklist_items items JOIN topics ON topics.id = items.topic_id
WHERE topics.slug = $2 AND items.id = $3
ON CONFLICT (user_id, checklist_item_id) DO UPDATE
SET completed = EXCLUDED.completed, updated_at = EXCLUDED.updated_at
RETURNING checklist_item_id, completed, updated_at`, userID, slug, itemID, completed, now)
	var result ChecklistProgress
	err := row.Scan(&result.ChecklistItemID, &result.Completed, &result.UpdatedAt)
	return result, err
}

func (store *PostgresStore) RecordTopicVisit(ctx context.Context, userID uuid.UUID, slug string, now time.Time) error {
	var topicID uuid.UUID
	return store.db.QueryRow(ctx, `
INSERT INTO topic_progress (user_id, topic_id, last_opened_at, updated_at)
SELECT $1, id, $3, $3 FROM topics WHERE slug = $2
ON CONFLICT (user_id, topic_id) DO UPDATE SET last_opened_at = EXCLUDED.last_opened_at
RETURNING topic_id`, userID, slug, now).Scan(&topicID)
}

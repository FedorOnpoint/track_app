package catalog

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type UserTopicParams struct {
	UserID                              uuid.UUID
	Query, Category, Difficulty, Status *string
	Bookmarked, ReviewRequired          *bool
	Sort                                string
	PageSize, PageOffset                int32
}

type UserTopic struct {
	ID                                  uuid.UUID
	Slug, Title, Summary, Difficulty    string
	EstimatedMinutes                    int32
	CategorySlug, CategoryTitle, Status string
	ReviewRequired, Bookmarked          bool
}

type PersonalizedStore interface {
	CountUserTopics(context.Context, UserTopicParams) (int64, error)
	ListUserTopics(context.Context, UserTopicParams) ([]UserTopic, error)
}

type personalizedDB interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}
type PostgresPersonalizedStore struct{ db personalizedDB }

func NewPostgresPersonalizedStore(db personalizedDB) *PostgresPersonalizedStore {
	return &PostgresPersonalizedStore{db: db}
}

const userTopicFromWhere = `
FROM topics t
JOIN categories c ON c.id=t.category_id
LEFT JOIN topic_progress p ON p.topic_id=t.id AND p.user_id=$1
LEFT JOIN bookmarks b ON b.topic_id=t.id AND b.user_id=$1
WHERE ($2::text IS NULL OR t.title ILIKE '%'||$2||'%' OR t.summary ILIKE '%'||$2||'%'
  OR EXISTS (SELECT 1 FROM topic_sections s WHERE s.topic_id=t.id AND s.body_markdown ILIKE '%'||$2||'%')
  OR EXISTS (SELECT 1 FROM interview_questions q WHERE q.topic_id=t.id AND (q.question ILIKE '%'||$2||'%' OR q.short_answer ILIKE '%'||$2||'%' OR q.detailed_answer ILIKE '%'||$2||'%'))
  OR EXISTS (SELECT 1 FROM checklist_items i WHERE i.topic_id=t.id AND i.text ILIKE '%'||$2||'%'))
AND ($3::text IS NULL OR c.slug=$3)
AND ($4::text IS NULL OR t.difficulty=$4)
AND ($5::text IS NULL OR coalesce(p.status,'not_started')=$5)
AND ($6::boolean IS NULL OR (b.user_id IS NOT NULL)=$6)
AND ($7::boolean IS NULL OR coalesce(p.review_required,false)=$7)`

func userTopicArgs(params UserTopicParams) []any {
	return []any{params.UserID, params.Query, params.Category, params.Difficulty, params.Status, params.Bookmarked, params.ReviewRequired}
}

func (store *PostgresPersonalizedStore) CountUserTopics(ctx context.Context, params UserTopicParams) (int64, error) {
	var total int64
	err := store.db.QueryRow(ctx, `SELECT count(*) `+userTopicFromWhere, userTopicArgs(params)...).Scan(&total)
	return total, err
}
func (store *PostgresPersonalizedStore) ListUserTopics(ctx context.Context, params UserTopicParams) ([]UserTopic, error) {
	query := `SELECT t.id,t.slug,t.title,t.summary,t.difficulty,t.estimated_minutes,c.slug,c.title,coalesce(p.status,'not_started'),coalesce(p.review_required,false),(b.user_id IS NOT NULL) ` + userTopicFromWhere + `
ORDER BY CASE WHEN $8='title' THEN t.title END,CASE WHEN $8='difficulty' THEN t.difficulty END,CASE WHEN $8='category' THEN c.sort_order END,t.sort_order,t.slug LIMIT $9 OFFSET $10`
	args := append(userTopicArgs(params), params.Sort, params.PageSize, params.PageOffset)
	rows, err := store.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]UserTopic, 0)
	for rows.Next() {
		var item UserTopic
		if err := rows.Scan(&item.ID, &item.Slug, &item.Title, &item.Summary, &item.Difficulty, &item.EstimatedMinutes, &item.CategorySlug, &item.CategoryTitle, &item.Status, &item.ReviewRequired, &item.Bookmarked); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

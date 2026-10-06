package dashboard

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type ProgressSummary struct{ Total, Mastered int64 }
type CategoryProgress struct {
	Slug, Title     string
	Total, Mastered int64
}
type TopicItem struct {
	Slug, Title, Status string
	ReviewRequired      bool
	UpdatedAt           time.Time
	LastOpenedAt        *time.Time
}
type NoteItem struct {
	ID                                  uuid.UUID
	TopicSlug, TopicTitle, BodyMarkdown string
	UpdatedAt                           time.Time
}
type Dashboard struct {
	Overall                    ProgressSummary
	Categories                 []CategoryProgress
	ReviewTopics, RecentTopics []TopicItem
	RecentNotes                []NoteItem
	ContinueTopic              *TopicItem
}

type Store interface {
	Get(context.Context, uuid.UUID) (Dashboard, error)
}
type database interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}
type PostgresStore struct{ db database }

func NewPostgresStore(db database) *PostgresStore { return &PostgresStore{db: db} }

func (store *PostgresStore) Get(ctx context.Context, userID uuid.UUID) (Dashboard, error) {
	var result Dashboard
	if err := store.db.QueryRow(ctx, `SELECT count(*), count(*) FILTER (WHERE coalesce(p.status, 'not_started') = 'mastered') FROM topics t LEFT JOIN topic_progress p ON p.topic_id=t.id AND p.user_id=$1`, userID).Scan(&result.Overall.Total, &result.Overall.Mastered); err != nil {
		return result, err
	}
	rows, err := store.db.Query(ctx, `SELECT c.slug,c.title,count(t.id),count(t.id) FILTER (WHERE p.status='mastered') FROM categories c LEFT JOIN topics t ON t.category_id=c.id LEFT JOIN topic_progress p ON p.topic_id=t.id AND p.user_id=$1 GROUP BY c.id,c.slug,c.title,c.sort_order ORDER BY c.sort_order,c.slug`, userID)
	if err != nil {
		return result, err
	}
	for rows.Next() {
		var item CategoryProgress
		if err := rows.Scan(&item.Slug, &item.Title, &item.Total, &item.Mastered); err != nil {
			rows.Close()
			return result, err
		}
		result.Categories = append(result.Categories, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return result, err
	}
	rows.Close()
	if result.ReviewTopics, err = store.topics(ctx, userID, `WHERE p.user_id=$1 AND (p.review_required OR p.status='review') ORDER BY p.updated_at DESC,t.slug LIMIT 10`); err != nil {
		return result, err
	}
	if result.RecentTopics, err = store.topics(ctx, userID, `WHERE p.user_id=$1 AND p.updated_at IS NOT NULL ORDER BY p.updated_at DESC,t.slug LIMIT 10`); err != nil {
		return result, err
	}
	rows, err = store.db.Query(ctx, `SELECT n.id,t.slug,t.title,n.body_markdown,n.updated_at FROM notes n JOIN topics t ON t.id=n.topic_id WHERE n.user_id=$1 ORDER BY n.updated_at DESC,n.id LIMIT 10`, userID)
	if err != nil {
		return result, err
	}
	for rows.Next() {
		var item NoteItem
		if err := rows.Scan(&item.ID, &item.TopicSlug, &item.TopicTitle, &item.BodyMarkdown, &item.UpdatedAt); err != nil {
			rows.Close()
			return result, err
		}
		result.RecentNotes = append(result.RecentNotes, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return result, err
	}
	rows.Close()
	var item TopicItem
	err = store.db.QueryRow(ctx, `SELECT t.slug,t.title,p.status,p.review_required,p.updated_at,p.last_opened_at FROM topic_progress p JOIN topics t ON t.id=p.topic_id WHERE p.user_id=$1 AND p.last_opened_at IS NOT NULL ORDER BY p.last_opened_at DESC,t.slug LIMIT 1`, userID).Scan(&item.Slug, &item.Title, &item.Status, &item.ReviewRequired, &item.UpdatedAt, &item.LastOpenedAt)
	if err == nil {
		result.ContinueTopic = &item
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return result, err
	}
	return result, nil
}

func (store *PostgresStore) topics(ctx context.Context, userID uuid.UUID, clause string) ([]TopicItem, error) {
	rows, err := store.db.Query(ctx, `SELECT t.slug,t.title,p.status,p.review_required,p.updated_at,p.last_opened_at FROM topic_progress p JOIN topics t ON t.id=p.topic_id `+clause, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]TopicItem, 0)
	for rows.Next() {
		var item TopicItem
		if err := rows.Scan(&item.Slug, &item.Title, &item.Status, &item.ReviewRequired, &item.UpdatedAt, &item.LastOpenedAt); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

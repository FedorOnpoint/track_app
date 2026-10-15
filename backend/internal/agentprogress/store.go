package agentprogress

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresStore struct{ pool *pgxpool.Pool }

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore { return &PostgresStore{pool: pool} }

func (s *PostgresStore) Topics(ctx context.Context, user uuid.UUID) ([]Topic, error) {
	var exists bool
	if err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id=$1)`, user).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrUserNotFound
	}
	rows, err := s.pool.Query(ctx, `
SELECT t.slug,t.title,COALESCE(p.status,'not_started'),COALESCE(p.review_required,false),
 COALESCE((SELECT jsonb_agg(jsonb_build_object('key',c.stable_key,'text',c.text,'completed',COALESCE(cp.completed,false)) ORDER BY c.sort_order,c.id)
 FROM checklist_items c LEFT JOIN checklist_progress cp ON cp.checklist_item_id=c.id AND cp.user_id=$1
 WHERE c.topic_id=t.id),'[]'::jsonb)
FROM topics t LEFT JOIN topic_progress p ON p.topic_id=t.id AND p.user_id=$1 ORDER BY t.slug`, user)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Topic{}
	for rows.Next() {
		var topic Topic
		var checklist []byte
		if err := rows.Scan(&topic.Slug, &topic.Title, &topic.Status, &topic.ReviewRequired, &checklist); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(checklist, &topic.Checklist); err != nil {
			return nil, err
		}
		result = append(result, topic)
	}
	return result, rows.Err()
}

func (s *PostgresStore) Apply(ctx context.Context, user uuid.UUID, slug, key string, input Update) (Result, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Result{}, err
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = tx.Rollback(cleanup)
	}()
	result, err := apply(ctx, tx, user, slug, key, input)
	if err != nil {
		return Result{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Result{}, err
	}
	return result, nil
}

func apply(ctx context.Context, tx pgx.Tx, user uuid.UUID, slug, key string, input Update) (Result, error) {
	var owner uuid.UUID
	if err := tx.QueryRow(ctx, `SELECT id FROM users WHERE id=$1 FOR KEY SHARE`, user).Scan(&owner); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Result{}, ErrUserNotFound
		}
		return Result{}, err
	}
	var topicID uuid.UUID
	if err := tx.QueryRow(ctx, `SELECT id FROM topics WHERE slug=$1 FOR KEY SHARE`, slug).Scan(&topicID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Result{}, ErrNotFound
		}
		return Result{}, err
	}
	// Serialize updates of this user's topic, including concurrent retries.
	if _, err := tx.Exec(ctx, `INSERT INTO topic_progress(user_id,topic_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, user, topicID); err != nil {
		return Result{}, err
	}
	result := Result{Slug: slug, UpdateID: key}
	if err := tx.QueryRow(ctx, `SELECT status,review_required FROM topic_progress WHERE user_id=$1 AND topic_id=$2 FOR UPDATE`, user, topicID).Scan(&result.Status, &result.ReviewRequired); err != nil {
		return Result{}, err
	}
	payload, err := json.Marshal(input)
	if err != nil {
		return Result{}, err
	}
	hash := sha256.Sum256(payload)
	var previousHash, previousResult []byte
	err = tx.QueryRow(ctx, `SELECT payload_hash,result FROM agent_progress_updates WHERE user_id=$1 AND topic_id=$2 AND update_id=$3`, user, topicID, key).Scan(&previousHash, &previousResult)
	if err == nil {
		if !bytes.Equal(hash[:], previousHash) {
			return Result{}, ErrConflict
		}
		if err := json.Unmarshal(previousResult, &result); err != nil {
			return Result{}, err
		}
		return result, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Result{}, err
	}
	// Progress promotion is monotonic; other APIs retain manual correction support.
	rank := map[string]int{"not_started": 0, "learning": 1, "review": 2, "mastered": 3}
	if rank[input.Status] > rank[result.Status] {
		result.Status = input.Status
	}
	if input.ReviewRequired != nil {
		result.ReviewRequired = *input.ReviewRequired
	}
	if _, err := tx.Exec(ctx, `UPDATE topic_progress SET status=$3,review_required=$4,updated_at=now() WHERE user_id=$1 AND topic_id=$2`, user, topicID, result.Status, result.ReviewRequired); err != nil {
		return Result{}, err
	}
	for stableKey, completed := range input.Checklist {
		tag, err := tx.Exec(ctx, `INSERT INTO checklist_progress(user_id,checklist_item_id,completed)
SELECT $1,id,$4 FROM checklist_items WHERE topic_id=$2 AND stable_key=$3
ON CONFLICT(user_id,checklist_item_id) DO UPDATE SET completed=EXCLUDED.completed,updated_at=now()`, user, topicID, stableKey, *completed)
		if err != nil {
			return Result{}, err
		}
		if tag.RowsAffected() != 1 {
			return Result{}, ErrNotFound
		}
	}
	if input.Summary != nil {
		var id uuid.UUID
		if err := tx.QueryRow(ctx, `INSERT INTO notes(user_id,topic_id,body_markdown) VALUES($1,$2,$3) RETURNING id`, user, topicID, *input.Summary).Scan(&id); err != nil {
			return Result{}, err
		}
		result.NoteID = &id
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		return Result{}, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO agent_progress_updates(user_id,topic_id,update_id,payload_hash,result) VALUES($1,$2,$3,$4,$5)`, user, topicID, key, hash[:], encoded)
	return result, err
}

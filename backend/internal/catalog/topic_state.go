package catalog

import (
	"context"

	"github.com/google/uuid"
)

type UserTopicState struct {
	Status         string
	ReviewRequired bool
	Bookmarked     bool
	Checklist      map[uuid.UUID]bool
}

type UserTopicStateStore interface {
	GetTopicState(context.Context, uuid.UUID, string) (UserTopicState, error)
}

func (store *PostgresPersonalizedStore) GetTopicState(ctx context.Context, userID uuid.UUID, slug string) (UserTopicState, error) {
	result := UserTopicState{Checklist: make(map[uuid.UUID]bool)}
	err := store.db.QueryRow(ctx, `
SELECT coalesce(p.status,'not_started'),coalesce(p.review_required,false),(b.user_id IS NOT NULL)
FROM topics t
LEFT JOIN topic_progress p ON p.topic_id=t.id AND p.user_id=$1
LEFT JOIN bookmarks b ON b.topic_id=t.id AND b.user_id=$1
WHERE t.slug=$2`, userID, slug).Scan(&result.Status, &result.ReviewRequired, &result.Bookmarked)
	if err != nil {
		return UserTopicState{}, err
	}
	rows, err := store.db.Query(ctx, `
SELECT i.id,coalesce(p.completed,false)
FROM checklist_items i
JOIN topics t ON t.id=i.topic_id
LEFT JOIN checklist_progress p ON p.checklist_item_id=i.id AND p.user_id=$1
WHERE t.slug=$2
ORDER BY i.sort_order,i.id`, userID, slug)
	if err != nil {
		return UserTopicState{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var itemID uuid.UUID
		var completed bool
		if err := rows.Scan(&itemID, &completed); err != nil {
			return UserTopicState{}, err
		}
		result.Checklist[itemID] = completed
	}
	return result, rows.Err()
}

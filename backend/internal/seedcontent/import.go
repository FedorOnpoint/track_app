package seedcontent

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func Import(ctx context.Context, pool *pgxpool.Pool, documents []Document) error {
	if len(documents) == 0 {
		return fmt.Errorf("no seed documents")
	}
	titles, categories := map[string]string{}, map[string]bool{}
	for _, document := range documents {
		if err := Validate(document); err != nil {
			return err
		}
		if categories[document.Category.Slug] {
			return fmt.Errorf("duplicate category: %s", document.Category.Slug)
		}
		categories[document.Category.Slug] = true
		for _, topic := range document.Topics {
			if _, exists := titles[topic.Slug]; exists {
				return fmt.Errorf("duplicate topic: %s", topic.Slug)
			}
			titles[topic.Slug] = topic.Title
		}
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(74190321)`); err != nil {
		return err
	}
	for _, document := range documents {
		var categoryID uuid.UUID
		category := document.Category
		var currentVersion int
		if err := tx.QueryRow(ctx, `SELECT coalesce(max(v.content_version),0) FROM catalog_seed_versions v JOIN categories c ON c.id=v.category_id WHERE c.slug=$1`, category.Slug).Scan(&currentVersion); err != nil {
			return err
		}
		if document.Version < currentVersion {
			return fmt.Errorf("category %s: content version %d is older than %d", category.Slug, document.Version, currentVersion)
		}
		if err := tx.QueryRow(ctx, `INSERT INTO categories(slug,title,description,sort_order) VALUES($1,$2,$3,$4) ON CONFLICT(slug) DO UPDATE SET title=EXCLUDED.title,description=EXCLUDED.description,sort_order=EXCLUDED.sort_order RETURNING id`, category.Slug, category.Title, category.Description, category.SortOrder).Scan(&categoryID); err != nil {
			return fmt.Errorf("category %s: %w", category.Slug, err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO catalog_seed_versions(category_id,content_version) VALUES($1,$2) ON CONFLICT(category_id) DO UPDATE SET content_version=EXCLUDED.content_version,updated_at=now()`, categoryID, document.Version); err != nil {
			return err
		}
		for _, topic := range document.Topics {
			if err := importTopic(ctx, tx, categoryID, document.Version, topic, titles); err != nil {
				return fmt.Errorf("topic %s: %w", topic.Slug, err)
			}
		}
	}
	return tx.Commit(ctx)
}

func importTopic(ctx context.Context, tx pgx.Tx, categoryID uuid.UUID, version int, topic Topic, titles map[string]string) error {
	var topicID uuid.UUID
	if err := tx.QueryRow(ctx, `INSERT INTO topics(category_id,slug,title,summary,difficulty,sort_order,estimated_minutes,content_version) VALUES($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT(slug) DO UPDATE SET category_id=EXCLUDED.category_id,title=EXCLUDED.title,summary=EXCLUDED.summary,difficulty=EXCLUDED.difficulty,sort_order=EXCLUDED.sort_order,estimated_minutes=EXCLUDED.estimated_minutes,content_version=EXCLUDED.content_version,updated_at=now() WHERE topics.content_version<=EXCLUDED.content_version RETURNING id`, categoryID, topic.Slug, topic.Title, topic.Summary, topic.Difficulty, topic.SortOrder, topic.EstimatedMinutes, version).Scan(&topicID); err != nil {
		return fmt.Errorf("upsert topic (older versions are rejected): %w", err)
	}
	prerequisites := "Для этой темы специальных prerequisites нет."
	if len(topic.Prerequisites) > 0 {
		items := make([]string, 0, len(topic.Prerequisites))
		for _, slug := range topic.Prerequisites {
			title := titles[slug]
			if title == "" {
				title = slug
			}
			items = append(items, fmt.Sprintf("- [%s](/topics/%s)", title, slug))
		}
		prerequisites = strings.Join(items, "\n")
	}
	sections := append([]Section{{Heading: "Перед началом", BodyMarkdown: prerequisites, SortOrder: 0}}, topic.Sections...)
	for _, table := range []string{"topic_sections", "resources", "practice_recommendations"} {
		if _, err := tx.Exec(ctx, "DELETE FROM "+table+" WHERE topic_id=$1", topicID); err != nil {
			return err
		}
	}
	for _, section := range sections {
		if _, err := tx.Exec(ctx, `INSERT INTO topic_sections(topic_id,heading,body_markdown,sort_order) VALUES($1,$2,$3,$4) ON CONFLICT(topic_id,sort_order) DO UPDATE SET heading=EXCLUDED.heading,body_markdown=EXCLUDED.body_markdown`, topicID, section.Heading, section.BodyMarkdown, section.SortOrder); err != nil {
			return err
		}
	}
	checklistOrder := 0
	for _, item := range topic.Checklist {
		checklistOrder = max(checklistOrder, item.SortOrder)
	}
	if err := reserveOrders(ctx, tx, "checklist_items", topicID, checklistOrder); err != nil {
		return err
	}
	for _, item := range topic.Checklist {
		if _, err := tx.Exec(ctx, `INSERT INTO checklist_items(topic_id,stable_key,text,sort_order) VALUES($1,$2,$3,$4) ON CONFLICT(topic_id,stable_key) DO UPDATE SET text=EXCLUDED.text,sort_order=EXCLUDED.sort_order`, topicID, item.StableKey, item.Text, item.SortOrder); err != nil {
			return err
		}
	}
	questionOrder := 0
	for _, question := range topic.Questions {
		questionOrder = max(questionOrder, question.SortOrder)
	}
	if err := reserveOrders(ctx, tx, "interview_questions", topicID, questionOrder); err != nil {
		return err
	}
	for _, question := range topic.Questions {
		var questionID uuid.UUID
		if err := tx.QueryRow(ctx, `INSERT INTO interview_questions(topic_id,stable_key,question,short_answer,detailed_answer,pitfalls,sort_order) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(topic_id,stable_key) DO UPDATE SET question=EXCLUDED.question,short_answer=EXCLUDED.short_answer,detailed_answer=EXCLUDED.detailed_answer,pitfalls=EXCLUDED.pitfalls,sort_order=EXCLUDED.sort_order RETURNING id`, topicID, question.StableKey, question.Question, question.ShortAnswer, question.DetailedAnswer, question.Pitfalls, question.SortOrder).Scan(&questionID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM follow_up_questions WHERE interview_question_id=$1`, questionID); err != nil {
			return err
		}
		for _, followUp := range question.FollowUps {
			if _, err := tx.Exec(ctx, `INSERT INTO follow_up_questions(interview_question_id,question,expected_answer,sort_order) VALUES($1,$2,$3,$4) ON CONFLICT(interview_question_id,sort_order) DO UPDATE SET question=EXCLUDED.question,expected_answer=EXCLUDED.expected_answer`, questionID, followUp.Question, followUp.ExpectedAnswer, followUp.SortOrder); err != nil {
				return err
			}
		}
	}
	for _, resource := range topic.Resources {
		if _, err := tx.Exec(ctx, `INSERT INTO resources(topic_id,title,url,resource_type,source_name,sort_order) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT(topic_id,sort_order) DO UPDATE SET title=EXCLUDED.title,url=EXCLUDED.url,resource_type=EXCLUDED.resource_type,source_name=EXCLUDED.source_name`, topicID, resource.Title, resource.URL, resource.ResourceType, resource.SourceName, resource.SortOrder); err != nil {
			return err
		}
	}
	for _, practice := range topic.Practice {
		if _, err := tx.Exec(ctx, `INSERT INTO practice_recommendations(topic_id,title,description,url,expected_outcome,sort_order) VALUES($1,$2,$3,NULLIF($4,''),$5,$6) ON CONFLICT(topic_id,sort_order) DO UPDATE SET title=EXCLUDED.title,description=EXCLUDED.description,url=EXCLUDED.url,expected_outcome=EXCLUDED.expected_outcome`, topicID, practice.Title, practice.Description, practice.URL, practice.ExpectedOutcome, practice.SortOrder); err != nil {
			return err
		}
	}
	return nil
}

// Move existing rows out of the desired order range before swapping stable keys.
// Rows absent from an update remain available; checklist progress must not cascade away.
func reserveOrders(ctx context.Context, tx pgx.Tx, table string, topicID uuid.UUID, desiredMax int) error {
	query := fmt.Sprintf(`WITH existing AS (SELECT id,row_number() OVER(ORDER BY sort_order,id) AS position FROM %s WHERE topic_id=$1), bounds AS (SELECT greatest(coalesce(max(sort_order),0),$2::integer)::bigint AS max_order FROM %s WHERE topic_id=$1) UPDATE %s item SET sort_order=bounds.max_order+existing.position FROM existing,bounds WHERE item.id=existing.id`, table, table, table)
	_, err := tx.Exec(ctx, query, topicID, desiredMax)
	return err
}

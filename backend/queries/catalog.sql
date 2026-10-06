-- name: ListCategories :many
SELECT id, slug, title, description, sort_order
FROM categories
ORDER BY sort_order, slug;

-- name: CountTopics :one
SELECT count(*)
FROM topics t
JOIN categories c ON c.id = t.category_id
WHERE (sqlc.narg('query')::text IS NULL OR t.title ILIKE '%' || sqlc.narg('query') || '%' OR t.summary ILIKE '%' || sqlc.narg('query') || '%')
  AND (sqlc.narg('category')::text IS NULL OR c.slug = sqlc.narg('category'))
  AND (sqlc.narg('difficulty')::text IS NULL OR t.difficulty = sqlc.narg('difficulty'));

-- name: ListTopics :many
SELECT t.id, t.slug, t.title, t.summary, t.difficulty, t.estimated_minutes,
       c.slug AS category_slug, c.title AS category_title
FROM topics t
JOIN categories c ON c.id = t.category_id
WHERE (sqlc.narg('query')::text IS NULL OR t.title ILIKE '%' || sqlc.narg('query') || '%' OR t.summary ILIKE '%' || sqlc.narg('query') || '%')
  AND (sqlc.narg('category')::text IS NULL OR c.slug = sqlc.narg('category'))
  AND (sqlc.narg('difficulty')::text IS NULL OR t.difficulty = sqlc.narg('difficulty'))
ORDER BY
  CASE WHEN sqlc.arg('sort')::text = 'title' THEN t.title END ASC,
  CASE WHEN sqlc.arg('sort')::text = 'difficulty' THEN t.difficulty END ASC,
  CASE WHEN sqlc.arg('sort')::text = 'category' THEN c.sort_order END ASC,
  t.sort_order ASC,
  t.slug ASC
LIMIT sqlc.arg('page_size') OFFSET sqlc.arg('page_offset');

-- name: GetTopic :one
SELECT t.id, t.slug, t.title, t.summary, t.difficulty, t.estimated_minutes,
       c.slug AS category_slug, c.title AS category_title
FROM topics t
JOIN categories c ON c.id = t.category_id
WHERE t.slug = $1;

-- name: ListTopicSections :many
SELECT id, heading, body_markdown, sort_order FROM topic_sections WHERE topic_id = $1 ORDER BY sort_order;

-- name: ListChecklistItems :many
SELECT id, stable_key, text, sort_order FROM checklist_items WHERE topic_id = $1 ORDER BY sort_order;

-- name: ListInterviewQuestions :many
SELECT id, stable_key, question, short_answer, detailed_answer, pitfalls, sort_order FROM interview_questions WHERE topic_id = $1 ORDER BY sort_order;

-- name: ListFollowUpQuestions :many
SELECT id, interview_question_id, question, expected_answer, sort_order
FROM follow_up_questions
WHERE interview_question_id = ANY($1::uuid[])
ORDER BY interview_question_id, sort_order;

-- name: ListResources :many
SELECT id, title, url, resource_type, source_name, sort_order FROM resources WHERE topic_id = $1 ORDER BY sort_order;

-- name: ListPracticeRecommendations :many
SELECT id, title, description, url, expected_outcome, sort_order FROM practice_recommendations WHERE topic_id = $1 ORDER BY sort_order;

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE categories (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    slug text NOT NULL UNIQUE CHECK (slug ~ '^[a-z0-9]+(?:-[a-z0-9]+)*$'),
    title text NOT NULL CHECK (length(title) BETWEEN 1 AND 120),
    description text NOT NULL DEFAULT '',
    sort_order integer NOT NULL CHECK (sort_order >= 0)
);

CREATE TABLE topics (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    category_id uuid NOT NULL REFERENCES categories(id) ON DELETE RESTRICT,
    slug text NOT NULL UNIQUE CHECK (slug ~ '^[a-z0-9]+(?:-[a-z0-9]+)*$'),
    title text NOT NULL CHECK (length(title) BETWEEN 1 AND 180),
    summary text NOT NULL,
    difficulty text NOT NULL CHECK (difficulty IN ('basic', 'intermediate', 'advanced')),
    sort_order integer NOT NULL CHECK (sort_order >= 0),
    estimated_minutes integer NOT NULL CHECK (estimated_minutes > 0),
    content_version integer NOT NULL DEFAULT 1 CHECK (content_version > 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE topic_sections (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    topic_id uuid NOT NULL REFERENCES topics(id) ON DELETE CASCADE,
    heading text NOT NULL,
    body_markdown text NOT NULL,
    sort_order integer NOT NULL CHECK (sort_order >= 0),
    UNIQUE (topic_id, sort_order)
);

CREATE TABLE checklist_items (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    topic_id uuid NOT NULL REFERENCES topics(id) ON DELETE CASCADE,
    stable_key text NOT NULL CHECK (stable_key ~ '^[a-z0-9]+(?:-[a-z0-9]+)*$'),
    text text NOT NULL,
    sort_order integer NOT NULL CHECK (sort_order >= 0),
    UNIQUE (topic_id, stable_key),
    UNIQUE (topic_id, sort_order)
);

CREATE TABLE interview_questions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    topic_id uuid NOT NULL REFERENCES topics(id) ON DELETE CASCADE,
    stable_key text NOT NULL CHECK (stable_key ~ '^[a-z0-9]+(?:-[a-z0-9]+)*$'),
    question text NOT NULL,
    short_answer text NOT NULL,
    detailed_answer text NOT NULL,
    pitfalls text NOT NULL DEFAULT '',
    sort_order integer NOT NULL CHECK (sort_order >= 0),
    UNIQUE (topic_id, stable_key),
    UNIQUE (topic_id, sort_order)
);

CREATE TABLE follow_up_questions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    interview_question_id uuid NOT NULL REFERENCES interview_questions(id) ON DELETE CASCADE,
    question text NOT NULL,
    expected_answer text NOT NULL,
    sort_order integer NOT NULL CHECK (sort_order >= 0),
    UNIQUE (interview_question_id, sort_order)
);

CREATE TABLE resources (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    topic_id uuid NOT NULL REFERENCES topics(id) ON DELETE CASCADE,
    title text NOT NULL,
    url text NOT NULL CHECK (url ~ '^https://'),
    resource_type text NOT NULL CHECK (resource_type IN ('documentation', 'article', 'video', 'book', 'course', 'tool')),
    source_name text NOT NULL,
    sort_order integer NOT NULL CHECK (sort_order >= 0),
    UNIQUE (topic_id, sort_order)
);

CREATE TABLE practice_recommendations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    topic_id uuid NOT NULL REFERENCES topics(id) ON DELETE CASCADE,
    title text NOT NULL,
    description text NOT NULL,
    url text CHECK (url IS NULL OR url ~ '^https://'),
    expected_outcome text NOT NULL,
    sort_order integer NOT NULL CHECK (sort_order >= 0),
    UNIQUE (topic_id, sort_order)
);

CREATE INDEX topics_category_order_idx ON topics (category_id, sort_order, slug);
CREATE INDEX topics_difficulty_order_idx ON topics (difficulty, sort_order, slug);
CREATE INDEX topic_sections_topic_order_idx ON topic_sections (topic_id, sort_order);
CREATE INDEX checklist_items_topic_order_idx ON checklist_items (topic_id, sort_order);
CREATE INDEX interview_questions_topic_order_idx ON interview_questions (topic_id, sort_order);
CREATE INDEX resources_topic_order_idx ON resources (topic_id, sort_order);
CREATE INDEX practice_recommendations_topic_order_idx ON practice_recommendations (topic_id, sort_order);

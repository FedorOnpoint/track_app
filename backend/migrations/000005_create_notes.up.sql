CREATE TABLE notes (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    topic_id uuid NOT NULL REFERENCES topics(id) ON DELETE CASCADE,
    body_markdown text NOT NULL CHECK (length(body_markdown) BETWEEN 1 AND 20000),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX notes_user_updated_idx ON notes (user_id, updated_at DESC, id);
CREATE INDEX notes_user_topic_updated_idx ON notes (user_id, topic_id, updated_at DESC);

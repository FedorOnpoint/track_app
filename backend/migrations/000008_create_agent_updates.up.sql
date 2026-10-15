CREATE TABLE agent_progress_updates (
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    topic_id uuid NOT NULL REFERENCES topics(id) ON DELETE CASCADE,
    update_id text NOT NULL CHECK (length(update_id) BETWEEN 1 AND 128),
    payload_hash bytea NOT NULL CHECK (octet_length(payload_hash) = 32),
    result jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, topic_id, update_id)
);

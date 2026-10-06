CREATE TABLE topic_progress (
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    topic_id uuid NOT NULL REFERENCES topics(id) ON DELETE CASCADE,
    status text NOT NULL DEFAULT 'not_started' CHECK (status IN ('not_started', 'learning', 'review', 'mastered')),
    review_required boolean NOT NULL DEFAULT false,
    last_opened_at timestamptz,
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, topic_id)
);

CREATE TABLE checklist_progress (
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    checklist_item_id uuid NOT NULL REFERENCES checklist_items(id) ON DELETE CASCADE,
    completed boolean NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, checklist_item_id)
);

CREATE INDEX topic_progress_user_status_idx ON topic_progress (user_id, status, updated_at DESC);
CREATE INDEX topic_progress_user_review_idx ON topic_progress (user_id, updated_at DESC) WHERE review_required;
CREATE INDEX checklist_progress_user_completed_idx ON checklist_progress (user_id, completed);

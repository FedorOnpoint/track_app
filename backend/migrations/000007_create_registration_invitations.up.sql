CREATE TABLE registration_invitations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    token_hash bytea NOT NULL UNIQUE CHECK (octet_length(token_hash) = 32),
    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL CHECK (expires_at > created_at),
    used_at timestamptz,
    used_by uuid REFERENCES users(id),
    attempts integer NOT NULL DEFAULT 0 CHECK (attempts BETWEEN 0 AND 10),
    attempts_reset_at timestamptz NOT NULL DEFAULT now(),
    CHECK ((used_at IS NULL) = (used_by IS NULL)),
    CHECK (used_at IS NULL OR used_at >= created_at)
);

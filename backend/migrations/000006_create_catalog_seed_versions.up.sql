CREATE TABLE catalog_seed_versions (
    category_id uuid PRIMARY KEY REFERENCES categories(id) ON DELETE CASCADE,
    content_version integer NOT NULL CHECK (content_version > 0),
    updated_at timestamptz NOT NULL DEFAULT now()
);

INSERT INTO catalog_seed_versions (category_id, content_version)
SELECT c.id, greatest(1, coalesce(max(t.content_version), 1))
FROM categories c
LEFT JOIN topics t ON t.category_id = c.id
GROUP BY c.id;

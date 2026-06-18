-- +goose Up
CREATE TABLE admin_search_history (
    id TEXT PRIMARY KEY DEFAULT gen_random_uuid()::TEXT,
    admin_user_id TEXT NOT NULL,
    field_key TEXT NOT NULL,
    value TEXT NOT NULL,
    value_normalized TEXT NOT NULL,
    usage_count INTEGER NOT NULL DEFAULT 1,
    first_used_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_used_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT admin_search_history_unique_value UNIQUE (admin_user_id, field_key, value_normalized)
);

CREATE INDEX admin_search_history_user_field_recent_idx
    ON admin_search_history (admin_user_id, field_key, last_used_at DESC, usage_count DESC);

-- +goose Down
DROP TABLE IF EXISTS admin_search_history;

-- +goose Up
CREATE TABLE admin_ai_provider_keys (
    provider TEXT PRIMARY KEY CHECK (provider IN ('tokengate', 'openai', 'anthropic')),
    enabled BOOLEAN NOT NULL DEFAULT false,
    api_key_ciphertext TEXT NOT NULL,
    key_tail TEXT NOT NULL,
    base_url TEXT NOT NULL,
    chat_model TEXT NOT NULL DEFAULT '',
    messages_model TEXT NOT NULL DEFAULT '',
    last_validated_at TIMESTAMPTZ,
    last_validation_status TEXT,
    last_validation_error TEXT,
    last_rotated_at TIMESTAMPTZ,
    created_by_admin_id TEXT,
    updated_by_admin_id TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE ai_surface_routing (
    surface TEXT PRIMARY KEY CHECK (surface IN ('post_assist', 'error_triage', 'app_review_ai')),
    provider TEXT NOT NULL REFERENCES admin_ai_provider_keys(provider) ON DELETE RESTRICT,
    client_kind TEXT NOT NULL CHECK (client_kind IN ('chat_completions', 'messages')),
    model_override TEXT NOT NULL DEFAULT '',
    created_by_admin_id TEXT,
    updated_by_admin_id TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX ai_surface_routing_provider_idx ON ai_surface_routing (provider);

CREATE TABLE admin_ai_provider_events (
    id BIGSERIAL PRIMARY KEY,
    provider TEXT CHECK (provider IN ('tokengate', 'openai', 'anthropic')),
    surface TEXT CHECK (surface IS NULL OR surface IN ('post_assist', 'error_triage', 'app_review_ai')),
    action TEXT NOT NULL,
    category TEXT NOT NULL DEFAULT 'config',
    actor_admin_id TEXT,
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX admin_ai_provider_events_created_idx ON admin_ai_provider_events (created_at DESC);
CREATE INDEX admin_ai_provider_events_provider_created_idx ON admin_ai_provider_events (provider, created_at DESC) WHERE provider IS NOT NULL;
CREATE INDEX admin_ai_provider_events_action_created_idx ON admin_ai_provider_events (action, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS admin_ai_provider_events;
DROP TABLE IF EXISTS ai_surface_routing;
DROP TABLE IF EXISTS admin_ai_provider_keys;

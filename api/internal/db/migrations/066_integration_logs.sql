-- +goose Up

CREATE TABLE integration_logs (
    id BIGSERIAL PRIMARY KEY,
    workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,

    ts TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    level TEXT NOT NULL CHECK (level IN ('debug', 'info', 'warn', 'error')),
    status TEXT NOT NULL CHECK (status IN ('success', 'warning', 'error')),
    category TEXT NOT NULL CHECK (category IN ('publishing', 'api_request', 'oauth', 'webhook', 'system')),
    action TEXT NOT NULL,
    source TEXT NOT NULL CHECK (source IN ('api', 'dashboard', 'worker', 'webhook', 'oauth')),

    message TEXT NOT NULL,

    request_id TEXT,
    trace_id TEXT,

    actor_user_id TEXT,
    actor_api_key_id TEXT,

    profile_id TEXT,
    social_account_id TEXT,
    post_id TEXT,
    platform_post_id TEXT,
    platform TEXT,

    endpoint TEXT,
    method TEXT,
    http_status_code INTEGER,
    remote_status_code INTEGER,
    duration_ms INTEGER,

    error_code TEXT,

    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    request_payload JSONB,
    response_payload JSONB,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_integration_logs_workspace_ts
    ON integration_logs (workspace_id, ts DESC);

CREATE INDEX idx_integration_logs_workspace_category_ts
    ON integration_logs (workspace_id, category, ts DESC);

CREATE INDEX idx_integration_logs_workspace_status_ts
    ON integration_logs (workspace_id, status, ts DESC);

CREATE INDEX idx_integration_logs_workspace_platform_ts
    ON integration_logs (workspace_id, platform, ts DESC);

CREATE INDEX idx_integration_logs_workspace_request_id
    ON integration_logs (workspace_id, request_id);

CREATE INDEX idx_integration_logs_workspace_post_id
    ON integration_logs (workspace_id, post_id);

CREATE INDEX idx_integration_logs_workspace_social_account_id_ts
    ON integration_logs (workspace_id, social_account_id, ts DESC);

CREATE INDEX idx_integration_logs_workspace_action_ts
    ON integration_logs (workspace_id, action, ts DESC);

CREATE INDEX idx_integration_logs_metadata_gin
    ON integration_logs USING GIN (metadata);

-- +goose Down

DROP TABLE IF EXISTS integration_logs;

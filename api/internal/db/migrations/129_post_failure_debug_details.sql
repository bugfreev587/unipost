-- +goose Up
-- unipost:safety reversible

-- Observability-owned diagnostic storage. Intentionally no reference
-- constraint is declared against social_post_results: creating one would take a lock on a
-- protected publishing table during deployment. Retention removes orphaned
-- diagnostic rows independently.
CREATE TABLE post_failure_debug_details (
  social_post_result_id TEXT PRIMARY KEY,
  workspace_id TEXT NOT NULL,
  debug_text TEXT NOT NULL CHECK (octet_length(debug_text) <= 65536),
  original_bytes BIGINT NOT NULL CHECK (original_bytes >= 0),
  stored_bytes INTEGER NOT NULL CHECK (
    stored_bytes >= 0
    AND stored_bytes = octet_length(debug_text)
  ),
  truncated BOOLEAN NOT NULL DEFAULT FALSE,
  redaction_version INTEGER NOT NULL DEFAULT 1 CHECK (redaction_version >= 1),
  captured_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX post_failure_debug_details_workspace_captured_idx
  ON post_failure_debug_details (workspace_id, captured_at DESC);

-- +goose Down

DROP TABLE IF EXISTS post_failure_debug_details;

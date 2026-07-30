-- +goose Up
-- unipost:safety reversible

-- Canonical API-key request events. These are observability-owned relations and
-- deliberately do not reference customer business tables. The composite key is
-- required by PostgreSQL for uniqueness on a range-partitioned relation.
CREATE TABLE api_request_events (
  occurred_at       TIMESTAMPTZ NOT NULL,
  id                TEXT NOT NULL CHECK (octet_length(id) <= 80),
  workspace_id      TEXT NOT NULL CHECK (octet_length(workspace_id) <= 255),
  api_key_id         TEXT NOT NULL CHECK (octet_length(api_key_id) <= 255),
  request_id         TEXT CHECK (octet_length(request_id) <= 128),
  trace_id           TEXT CHECK (trace_id ~ '^[0-9a-f]{16,32}$'),
  method             TEXT NOT NULL CHECK (method ~ '^[A-Z]{1,16}$'),
  route_pattern      TEXT NOT NULL CHECK (route_pattern LIKE '/%' AND octet_length(route_pattern) <= 256),
  status_code        INTEGER NOT NULL CHECK (status_code BETWEEN 100 AND 599),
  duration_ms        INTEGER NOT NULL CHECK (duration_ms >= 0),
  outcome            TEXT NOT NULL CHECK (outcome IN (
    'success',
    'client_error',
    'validation_error',
    'authentication_error',
    'plan_or_quota_gate',
    'authorization_error',
    'not_found',
    'conflict',
    'rate_limited',
    'server_error'
  )),
  error_code         TEXT CHECK (error_code ~ '^[a-z][a-z0-9_]{0,63}$'),
  profile_id         TEXT CHECK (octet_length(profile_id) <= 255),
  social_account_id  TEXT CHECK (octet_length(social_account_id) <= 255),
  post_id             TEXT CHECK (octet_length(post_id) <= 255),
  has_error_detail   BOOLEAN NOT NULL DEFAULT FALSE,
  created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (occurred_at, id, workspace_id)
) PARTITION BY RANGE (occurred_at);

CREATE TABLE api_request_events_2026w31
  PARTITION OF api_request_events
  FOR VALUES FROM ('2026-07-27 00:00:00+00') TO ('2026-08-03 00:00:00+00');

CREATE TABLE api_request_events_2026w32
  PARTITION OF api_request_events
  FOR VALUES FROM ('2026-08-03 00:00:00+00') TO ('2026-08-10 00:00:00+00');

CREATE TABLE api_request_events_default
  PARTITION OF api_request_events DEFAULT;

CREATE INDEX api_request_events_workspace_time_idx
  ON api_request_events (workspace_id, occurred_at DESC, id DESC);

CREATE INDEX api_request_events_admin_time_idx
  ON api_request_events (occurred_at DESC, id DESC);

CREATE INDEX api_request_events_workspace_request_idx
  ON api_request_events (workspace_id, request_id)
  WHERE request_id IS NOT NULL;

CREATE INDEX api_request_events_workspace_route_time_idx
  ON api_request_events (workspace_id, route_pattern, occurred_at DESC, id DESC);

-- Failure-only bounded detail. Event timestamp is part of the identity so the
-- foreign key remains valid across aligned partitions.
CREATE TABLE api_request_error_details (
  occurred_at              TIMESTAMPTZ NOT NULL,
  event_id                 TEXT NOT NULL,
  workspace_id             TEXT NOT NULL,
  request_headers          JSONB NOT NULL DEFAULT '{}'::JSONB,
  response_headers         JSONB NOT NULL DEFAULT '{}'::JSONB,
  query_keys               TEXT[] NOT NULL DEFAULT '{}'::TEXT[],
  request_payload          TEXT,
  response_payload         TEXT,
  request_original_bytes   BIGINT NOT NULL DEFAULT 0 CHECK (request_original_bytes >= 0),
  request_stored_bytes     INTEGER NOT NULL DEFAULT 0 CHECK (request_stored_bytes >= 0),
  response_original_bytes  BIGINT NOT NULL DEFAULT 0 CHECK (response_original_bytes >= 0),
  response_stored_bytes    INTEGER NOT NULL DEFAULT 0 CHECK (response_stored_bytes >= 0),
  request_content_type     TEXT,
  response_content_type    TEXT,
  request_sha256           TEXT,
  response_sha256          TEXT,
  request_truncated        BOOLEAN NOT NULL DEFAULT FALSE,
  response_truncated       BOOLEAN NOT NULL DEFAULT FALSE,
  request_omission_reason  TEXT,
  response_omission_reason TEXT,
  redaction_policy_version INTEGER NOT NULL CHECK (redaction_policy_version >= 1),
  created_at               TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (occurred_at, event_id, workspace_id),
  FOREIGN KEY (occurred_at, event_id, workspace_id)
    REFERENCES api_request_events (occurred_at, id, workspace_id)
    ON DELETE CASCADE,
  CHECK (octet_length(request_payload) <= 32768),
  CHECK (octet_length(response_payload) <= 32768),
  CHECK (request_stored_bytes = octet_length(COALESCE(request_payload, ''))),
  CHECK (response_stored_bytes = octet_length(COALESCE(response_payload, ''))),
  CHECK (octet_length(COALESCE(request_content_type, '')) <= 255),
  CHECK (octet_length(COALESCE(response_content_type, '')) <= 255),
  CHECK (request_sha256 IS NULL OR request_sha256 ~ '^[0-9a-f]{64}$'),
  CHECK (response_sha256 IS NULL OR response_sha256 ~ '^[0-9a-f]{64}$'),
  CHECK (octet_length(COALESCE(request_omission_reason, '')) <= 128),
  CHECK (octet_length(COALESCE(response_omission_reason, '')) <= 128),
  CHECK (
    octet_length(event_id)
    + octet_length(workspace_id)
    + octet_length(COALESCE(request_payload, ''))
    + octet_length(COALESCE(response_payload, ''))
    + octet_length(request_headers::TEXT)
    + octet_length(response_headers::TEXT)
    + octet_length(query_keys::TEXT)
    + octet_length(COALESCE(request_content_type, ''))
    + octet_length(COALESCE(response_content_type, ''))
    + octet_length(COALESCE(request_sha256, ''))
    + octet_length(COALESCE(response_sha256, ''))
    + octet_length(COALESCE(request_omission_reason, ''))
    + octet_length(COALESCE(response_omission_reason, ''))
    <= 71680
  )
) PARTITION BY RANGE (occurred_at);

CREATE TABLE api_request_error_details_2026w31
  PARTITION OF api_request_error_details
  FOR VALUES FROM ('2026-07-27 00:00:00+00') TO ('2026-08-03 00:00:00+00');

CREATE TABLE api_request_error_details_2026w32
  PARTITION OF api_request_error_details
  FOR VALUES FROM ('2026-08-03 00:00:00+00') TO ('2026-08-10 00:00:00+00');

CREATE TABLE api_request_error_details_default
  PARTITION OF api_request_error_details DEFAULT;

CREATE INDEX api_request_error_details_workspace_time_idx
  ON api_request_error_details (workspace_id, occurred_at DESC, event_id DESC);

-- The relation is introduced in this workstream. Its idempotent population and
-- percentile read path are implemented in the read-migration workstream.
CREATE TABLE api_request_metric_rollups_hourly (
  hour_bucket            TIMESTAMPTZ NOT NULL,
  workspace_id           TEXT NOT NULL,
  route_pattern          TEXT NOT NULL,
  method                 TEXT NOT NULL,
  status_code            INTEGER NOT NULL CHECK (status_code BETWEEN 100 AND 599),
  request_count          BIGINT NOT NULL CHECK (request_count >= 0),
  error_count            BIGINT NOT NULL CHECK (error_count >= 0),
  server_error_count     BIGINT NOT NULL CHECK (server_error_count >= 0),
  validation_error_count BIGINT NOT NULL CHECK (validation_error_count >= 0),
  rate_limit_count       BIGINT NOT NULL CHECK (rate_limit_count >= 0),
  duration_sum_ms        BIGINT NOT NULL CHECK (duration_sum_ms >= 0),
  duration_min_ms        INTEGER NOT NULL CHECK (duration_min_ms >= 0),
  duration_max_ms        INTEGER NOT NULL CHECK (duration_max_ms >= duration_min_ms),
  duration_histogram     BIGINT[] NOT NULL CHECK (cardinality(duration_histogram) = 12),
  updated_at             TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (hour_bucket, workspace_id, route_pattern, method, status_code)
);

CREATE INDEX api_request_metric_rollups_workspace_hour_idx
  ON api_request_metric_rollups_hourly (workspace_id, hour_bucket DESC);

-- Workstream 4 owns detach/drop automation. This manifest gives that operation
-- an explicit parent/detail boundary contract instead of relying on names.
CREATE TABLE api_request_partition_manifest (
  week_start       TIMESTAMPTZ PRIMARY KEY,
  week_end         TIMESTAMPTZ NOT NULL UNIQUE,
  event_partition  TEXT NOT NULL UNIQUE,
  detail_partition TEXT NOT NULL UNIQUE,
  state            TEXT NOT NULL DEFAULT 'active' CHECK (state IN ('active', 'closed', 'detached')),
  created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CHECK (week_end = week_start + INTERVAL '7 days')
);

INSERT INTO api_request_partition_manifest (
  week_start, week_end, event_partition, detail_partition
) VALUES
  ('2026-07-27 00:00:00+00', '2026-08-03 00:00:00+00', 'api_request_events_2026w31', 'api_request_error_details_2026w31'),
  ('2026-08-03 00:00:00+00', '2026-08-10 00:00:00+00', 'api_request_events_2026w32', 'api_request_error_details_2026w32');

-- Internal, environment-global read kill-switch. This workstream registers it
-- OFF but does not evaluate it or switch any read path.
ALTER TABLE feature_flags
  DROP CONSTRAINT feature_flags_key_check;

ALTER TABLE feature_flags
  ADD CONSTRAINT feature_flags_key_check CHECK (
    key IN ('x_dms_v1', 'x_credits_billing_v1', 'observability_reads_v2')
  );

INSERT INTO feature_flags (key, enabled, description)
VALUES (
  'observability_reads_v2',
  FALSE,
  'Uses the canonical request-event model for Logs, Metrics, and Errors reads.'
)
ON CONFLICT (key) DO NOTHING;

-- +goose Down

-- Preserve the seeded flag and any immutable feature_flag_changes audit rows.
-- Older binaries ignore unknown definitions, so retaining this OFF control row
-- is safer than deleting security audit history during an application rollback.

DROP TABLE IF EXISTS api_request_partition_manifest;
DROP TABLE IF EXISTS api_request_metric_rollups_hourly;
DROP TABLE IF EXISTS api_request_error_details;
DROP TABLE IF EXISTS api_request_events;

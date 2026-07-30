-- +goose Up
-- unipost:safety reversible

-- A completion row distinguishes a successfully recomputed zero-event hour
-- from an hour whose rollup job has not finished. Metrics readers use this
-- observability-only marker to expose an explicit delayed state.
CREATE TABLE api_request_metric_rollup_hours (
  hour_bucket TIMESTAMPTZ PRIMARY KEY,
  completed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CHECK (
    hour_bucket = (
      date_trunc('hour', hour_bucket AT TIME ZONE 'UTC') AT TIME ZONE 'UTC'
    )
  )
);

-- +goose Down
DROP TABLE IF EXISTS api_request_metric_rollup_hours;

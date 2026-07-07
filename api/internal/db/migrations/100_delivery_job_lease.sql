-- +goose Up
--
-- Delivery job lease/heartbeat.
--
-- Stale recovery previously reaped any running/retrying job whose
-- last_attempt_at (written at CLAIM time) was older than a static 5m.
-- That misfires when a worker claims a batch and processes it serially:
-- jobs waiting their turn look "stalled" even though the worker is alive,
-- so recovery re-queues them and the platform gets a duplicate publish.
--
-- The lease decouples "worker alive and owns this job" from wall-clock.
-- The worker sets lease_expires_at at claim and renews it (heartbeat)
-- while it still owns the job; stale recovery only reaps jobs whose lease
-- has actually expired (worker died / never renewed).

ALTER TABLE post_delivery_jobs
  ADD COLUMN lease_expires_at TIMESTAMPTZ,
  ADD COLUMN lease_owner      TEXT;

-- Reap lookup: active jobs ordered by lease expiry.
CREATE INDEX post_delivery_jobs_lease_expiry_idx
  ON post_delivery_jobs (lease_expires_at)
  WHERE state IN ('running', 'retrying');

-- +goose Down
DROP INDEX IF EXISTS post_delivery_jobs_lease_expiry_idx;
ALTER TABLE post_delivery_jobs DROP COLUMN IF EXISTS lease_owner;
ALTER TABLE post_delivery_jobs DROP COLUMN IF EXISTS lease_expires_at;

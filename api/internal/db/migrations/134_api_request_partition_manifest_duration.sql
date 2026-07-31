-- +goose Up
-- unipost:safety reversible

-- TIMESTAMPTZ + INTERVAL '7 days' follows the session time zone's calendar.
-- Compare elapsed seconds instead so every manifest week is exactly 168 hours,
-- including weeks that cross a daylight-saving transition.
ALTER TABLE api_request_partition_manifest
  DROP CONSTRAINT api_request_partition_manifest_check;

ALTER TABLE api_request_partition_manifest
  ADD CONSTRAINT api_request_partition_manifest_week_duration_check
  CHECK (EXTRACT(EPOCH FROM (week_end - week_start)) = 604800);

-- +goose Down

ALTER TABLE api_request_partition_manifest
  DROP CONSTRAINT api_request_partition_manifest_week_duration_check;

ALTER TABLE api_request_partition_manifest
  ADD CONSTRAINT api_request_partition_manifest_check
  CHECK (week_end = week_start + INTERVAL '7 days');

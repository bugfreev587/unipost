-- +goose Up

-- debug_curl stores the curl-equivalent of every failing outbound HTTP
-- request captured during a publish attempt. Populated only when status
-- is 'failed'; nullable for rows created before this column existed and
-- for successful publishes (where there's nothing to debug).
--
-- TEXT with no hard cap — individual entries are bounded by the capture
-- layer (8KB response body, 16 entries per attempt), so practical row
-- size stays well under a page.
ALTER TABLE social_post_results
  ADD COLUMN debug_curl TEXT;

-- +goose Down

ALTER TABLE social_post_results
  DROP COLUMN debug_curl;

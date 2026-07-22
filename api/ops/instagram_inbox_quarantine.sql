\set ON_ERROR_STOP on

\if :{?incident_key}
\else
  \echo 'incident_key is required'
  \quit
\endif

\if :{?apply}
\else
  \set apply false
\endif

\if :apply
  \if :{?expected_count}
  \else
    \echo 'expected_count is required when apply=true; use the immediately preceding dry run'
    \quit
  \endif
  \if :{?expected_digest}
  \else
    \echo 'expected_digest is required when apply=true; use the immediately preceding dry run'
    \quit
  \endif
  \if :{?recovery_ready}
  \else
    \echo 'recovery_ready=true is required when apply=true after snapshot/PITR confirmation'
    \quit
  \endif
  \if :recovery_ready
  \else
    \echo 'recovery_ready must be true when apply=true'
    \quit
  \endif
\endif

BEGIN ISOLATION LEVEL REPEATABLE READ;

SELECT pg_advisory_xact_lock(hashtextextended('unipost:instagram-inbox-quarantine', 0));

CREATE TEMP TABLE instagram_inbox_quarantine_duplicate_keys
ON COMMIT DROP
AS
SELECT
  i.source,
  i.external_id
FROM inbox_items i
JOIN social_accounts sa ON sa.id = i.social_account_id
WHERE sa.platform = 'instagram'
  AND i.source IN ('ig_comment', 'ig_dm')
GROUP BY i.source, i.external_id
HAVING COUNT(DISTINCT sa.external_account_id) > 1;

CREATE TEMP TABLE instagram_inbox_quarantine_candidates (
  id                  TEXT PRIMARY KEY,
  source              TEXT NOT NULL,
  external_id         TEXT NOT NULL,
  social_account_id   TEXT NOT NULL,
  workspace_id        TEXT NOT NULL,
  account_external_id TEXT NOT NULL,
  original_row        JSONB NOT NULL
) ON COMMIT DROP;

INSERT INTO instagram_inbox_quarantine_candidates (
  id,
  source,
  external_id,
  social_account_id,
  workspace_id,
  account_external_id,
  original_row
)
SELECT
  i.id,
  i.source,
  i.external_id,
  i.social_account_id,
  i.workspace_id,
  sa.external_account_id,
  to_jsonb(i) AS original_row
FROM inbox_items i
JOIN social_accounts sa ON sa.id = i.social_account_id
JOIN instagram_inbox_quarantine_duplicate_keys duplicate_key
  ON duplicate_key.source = i.source
 AND duplicate_key.external_id = i.external_id
WHERE sa.platform = 'instagram'
  AND i.source IN ('ig_comment', 'ig_dm')
ORDER BY i.id
FOR UPDATE OF i;

SELECT
  COUNT(*)::BIGINT AS candidate_count,
  MD5(COALESCE(STRING_AGG(id, ',' ORDER BY id), '')) AS candidate_digest
FROM instagram_inbox_quarantine_candidates
\gset

\echo 'Instagram Inbox quarantine candidate summary (no message content):'
SELECT
  :candidate_count::BIGINT AS candidate_count,
  :'candidate_digest'::TEXT AS candidate_digest;

\if :apply
  SELECT (:candidate_count::BIGINT = :expected_count::BIGINT) AS candidate_count_matches
  \gset
  SELECT (:'candidate_digest'::TEXT = :'expected_digest'::TEXT) AS candidate_digest_matches
  \gset

  \if :candidate_count_matches
  \else
    \echo 'candidate count changed; rolling back without moving rows'
    ROLLBACK;
    \quit
  \endif

  \if :candidate_digest_matches
  \else
    \echo 'candidate digest changed; rolling back without moving rows'
    ROLLBACK;
    \quit
  \endif

  WITH preserved AS (
    INSERT INTO inbox_item_quarantine (
      incident_key,
      original_inbox_item_id,
      source,
      external_id,
      social_account_id,
      workspace_id,
      account_external_id,
      original_row
    )
    SELECT
      :'incident_key'::TEXT,
      candidate.id,
      candidate.source,
      candidate.external_id,
      candidate.social_account_id,
      candidate.workspace_id,
      candidate.account_external_id,
      candidate.original_row
    FROM instagram_inbox_quarantine_candidates candidate
    ORDER BY candidate.id
    ON CONFLICT (incident_key, original_inbox_item_id) DO NOTHING
    RETURNING original_inbox_item_id
  )
  SELECT COUNT(*)::BIGINT AS preserved_count
  FROM preserved
  \gset

  SELECT (:preserved_count::BIGINT = :candidate_count::BIGINT) AS preserved_count_matches
  \gset
  \if :preserved_count_matches
  \else
    \echo 'preserved count mismatch; rolling back without deleting live rows'
    ROLLBACK;
    \quit
  \endif

  WITH deleted AS (
    DELETE FROM inbox_items live_item
    USING instagram_inbox_quarantine_candidates candidate
    WHERE live_item.id = candidate.id
    RETURNING live_item.id
  )
  SELECT COUNT(*)::BIGINT AS deleted_count
  FROM deleted
  \gset

  SELECT (:deleted_count::BIGINT = :candidate_count::BIGINT) AS deleted_count_matches
  \gset
  \if :deleted_count_matches
  \else
    \echo 'deleted count mismatch; rolling back evidence and live-row changes'
    ROLLBACK;
    \quit
  \endif

  SELECT COUNT(*)::BIGINT AS remaining_count
  FROM inbox_items live_item
  JOIN instagram_inbox_quarantine_candidates candidate ON candidate.id = live_item.id
  \gset

  SELECT (:remaining_count::BIGINT = 0) AS remaining_count_matches
  \gset
  \if :remaining_count_matches
  \else
    \echo 'selected live rows remain; rolling back evidence and live-row changes'
    ROLLBACK;
    \quit
  \endif

  SELECT
    :candidate_count::BIGINT AS candidate_count,
    :preserved_count::BIGINT AS preserved_count,
    :deleted_count::BIGINT AS deleted_count,
    :remaining_count::BIGINT AS remaining_count,
    :'candidate_digest'::TEXT AS candidate_digest;

  COMMIT;
  \echo 'Instagram Inbox quarantine committed.'
\else
  ROLLBACK;
  \echo 'Dry run complete. No rows were changed.'
\endif

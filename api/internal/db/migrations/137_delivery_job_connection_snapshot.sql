-- +goose Up
-- unipost:safety reversible
ALTER TABLE post_delivery_jobs
  ADD COLUMN connection_id TEXT REFERENCES social_connections(id) ON DELETE SET NULL,
  ADD COLUMN binding_version BIGINT;

-- A logical Post may contain multiple thread segments for one public binding,
-- but it may select only one public binding for a physical destination. The
-- reservation survives terminal jobs so a later retry path cannot switch to a
-- sibling binding and publish the same logical Post twice.
CREATE TABLE social_post_physical_targets (
  post_id                    TEXT NOT NULL REFERENCES social_posts(id) ON DELETE CASCADE,
  physical_target_key        TEXT NOT NULL,
  connection_id              TEXT REFERENCES social_connections(id) ON DELETE SET NULL,
  selected_social_account_id TEXT NOT NULL,
  status                     TEXT NOT NULL DEFAULT 'active'
    CHECK (status IN ('active', 'migration_conflict')),
  conflict_details           JSONB NOT NULL DEFAULT '{}'::JSONB,
  created_at                 TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at                 TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (post_id, physical_target_key),
  CHECK (
    physical_target_key LIKE 'connection:%'
    OR physical_target_key LIKE 'legacy-account:%'
  )
);

CREATE INDEX social_post_physical_targets_connection_idx
  ON social_post_physical_targets (connection_id, created_at)
  WHERE connection_id IS NOT NULL;

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION reserve_post_delivery_job_physical_target()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
  target_key TEXT;
  reserved social_post_physical_targets%ROWTYPE;
BEGIN
  target_key := CASE
    WHEN NEW.connection_id IS NULL
      THEN 'legacy-account:' || NEW.social_account_id
    ELSE 'connection:' || NEW.connection_id
  END;

  INSERT INTO social_post_physical_targets (
    post_id,
    physical_target_key,
    connection_id,
    selected_social_account_id,
    status,
    conflict_details
  ) VALUES (
    NEW.post_id,
    target_key,
    NEW.connection_id,
    NEW.social_account_id,
    'active',
    '{}'::JSONB
  )
  ON CONFLICT (post_id, physical_target_key) DO NOTHING;

  SELECT * INTO STRICT reserved
  FROM social_post_physical_targets
  WHERE post_id = NEW.post_id
    AND physical_target_key = target_key
  FOR UPDATE;

  IF reserved.status <> 'active'
     OR reserved.selected_social_account_id <> NEW.social_account_id THEN
    RAISE EXCEPTION 'post physical target already selected through another binding'
      USING
        ERRCODE = '23514',
        CONSTRAINT = 'social_post_physical_target_binding_check';
  END IF;

  RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER post_delivery_jobs_reserve_physical_target
BEFORE INSERT ON post_delivery_jobs
FOR EACH ROW
EXECUTE FUNCTION reserve_post_delivery_job_physical_target();

-- The rollout phase is a cross-version claim gate. Returning NULL keeps old
-- claimers compatible while preventing a pending job from acquiring a new
-- provider-I/O lease during drain. Existing leases may renew and finalize.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION guard_post_delivery_job_claim_during_social_connection_cutover()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
  rollout_phase TEXT;
BEGIN
  SELECT phase INTO STRICT rollout_phase
  FROM social_connection_rollout_state
  WHERE id = 'global';

  IF rollout_phase IN ('draining', 'cutting_over')
     AND NEW.state IN ('running', 'retrying')
     AND (
       OLD.state = 'pending'
       OR OLD.lease_owner IS DISTINCT FROM NEW.lease_owner
     ) THEN
    RETURN NULL;
  END IF;

  RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER post_delivery_jobs_cutover_claim_guard
BEFORE UPDATE ON post_delivery_jobs
FOR EACH ROW
EXECUTE FUNCTION guard_post_delivery_job_claim_during_social_connection_cutover();

-- Reserve the complete logical Post target set in one statement. Any sibling
-- conflict raises and rolls back every reservation made by this call.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION reserve_social_post_physical_targets(
  requested_post_id TEXT,
  requested_account_ids TEXT[],
  requested_connection_ids TEXT[]
)
RETURNS VOID
LANGUAGE plpgsql
AS $$
DECLARE
  requested RECORD;
  reserved social_post_physical_targets%ROWTYPE;
BEGIN
  IF CARDINALITY(requested_account_ids) <> CARDINALITY(requested_connection_ids) THEN
    RAISE EXCEPTION 'physical target account and connection arrays must align'
      USING ERRCODE = '2202E';
  END IF;

  FOR requested IN
    SELECT DISTINCT
      input.account_id,
      NULLIF(input.connection_id, '') AS connection_id,
      CASE
        WHEN NULLIF(input.connection_id, '') IS NULL
          THEN 'legacy-account:' || input.account_id
        ELSE 'connection:' || NULLIF(input.connection_id, '')
      END AS target_key
    FROM UNNEST(requested_account_ids, requested_connection_ids)
      AS input(account_id, connection_id)
    ORDER BY target_key, account_id
  LOOP
    INSERT INTO social_post_physical_targets (
      post_id,
      physical_target_key,
      connection_id,
      selected_social_account_id,
      status,
      conflict_details
    ) VALUES (
      requested_post_id,
      requested.target_key,
      requested.connection_id,
      requested.account_id,
      'active',
      '{}'::JSONB
    )
    ON CONFLICT (post_id, physical_target_key) DO NOTHING;

    SELECT * INTO STRICT reserved
    FROM social_post_physical_targets
    WHERE post_id = requested_post_id
      AND physical_target_key = requested.target_key
    FOR UPDATE;

    IF reserved.status <> 'active'
       OR reserved.selected_social_account_id <> requested.account_id THEN
      RAISE EXCEPTION 'post physical target already selected through another binding'
        USING
          ERRCODE = '23514',
          CONSTRAINT = 'social_post_physical_target_binding_check';
    END IF;
  END LOOP;
END;
$$;
-- +goose StatementEnd

CREATE INDEX post_delivery_jobs_physical_connection_active_idx
  ON post_delivery_jobs ((COALESCE(connection_id, social_account_id)), created_at, id)
  WHERE state IN ('pending', 'running', 'retrying');

-- +goose Down
DROP TRIGGER IF EXISTS post_delivery_jobs_cutover_claim_guard ON post_delivery_jobs;
DROP FUNCTION IF EXISTS guard_post_delivery_job_claim_during_social_connection_cutover();
DROP TRIGGER IF EXISTS post_delivery_jobs_reserve_physical_target ON post_delivery_jobs;
DROP FUNCTION IF EXISTS reserve_post_delivery_job_physical_target();
DROP FUNCTION IF EXISTS reserve_social_post_physical_targets(TEXT, TEXT[], TEXT[]);
DROP INDEX IF EXISTS post_delivery_jobs_physical_connection_active_idx;
DROP INDEX IF EXISTS social_post_physical_targets_connection_idx;
DROP TABLE IF EXISTS social_post_physical_targets;

ALTER TABLE post_delivery_jobs
  DROP COLUMN IF EXISTS binding_version,
  DROP COLUMN IF EXISTS connection_id;

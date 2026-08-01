-- Forward-only, backup-gated authority reconciliation.
-- This file is embedded and executed in one PostgreSQL transaction.
SET LOCAL lock_timeout = '30s';
SET LOCAL statement_timeout = '15min';

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM social_connection_rollout_state
    WHERE id = 'global' AND phase = 'cutting_over'
  ) THEN
    RAISE EXCEPTION 'social connection cutover requires cutting_over phase';
  END IF;
END;
$$;

UPDATE social_connection_rollout_state
SET cutover_backend_pid = pg_backend_pid(), updated_at = NOW()
WHERE id = 'global' AND phase = 'cutting_over';

LOCK TABLE social_accounts IN SHARE ROW EXCLUSIVE MODE;
LOCK TABLE post_delivery_jobs IN SHARE ROW EXCLUSIVE MODE;
LOCK TABLE social_post_results IN SHARE ROW EXCLUSIVE MODE;
LOCK TABLE inbox_items IN SHARE ROW EXCLUSIVE MODE;
LOCK TABLE physical_daily_publish_operations IN SHARE ROW EXCLUSIVE MODE;
LOCK TABLE physical_daily_publish_reservations IN SHARE ROW EXCLUSIVE MODE;

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM post_delivery_jobs WHERE state IN ('running', 'retrying')) THEN
    RAISE EXCEPTION 'provider delivery leases appeared during cutover';
  END IF;
END;
$$;

-- Missing canonical identities are recorded per source row. Instagram must use
-- its professional/webhook user ID; its application-domain external_account_id
-- is not a safe substitute. Bluesky and every other current provider use the
-- verified external_account_id (the Bluesky value is its DID).
WITH source AS (
  SELECT
    sa.id AS account_id,
    sa.profile_id,
    sa.connection_id,
    p.workspace_id,
    sa.platform,
    sa.connection_type,
    sa.external_user_id,
    (sa.disconnected_at IS NULL AND sa.status <> 'disconnected') AS is_live,
    CASE
      WHEN sa.platform = 'instagram'
        THEN NULLIF(BTRIM(sa.metadata->>'instagram_webhook_user_id'), '')
      WHEN sa.platform <> 'instagram'
        THEN NULLIF(BTRIM(sa.external_account_id), '')
    END AS provider_identity
  FROM social_accounts sa
  JOIN profiles p ON p.id = sa.profile_id
)
INSERT INTO social_connection_migration_conflicts (
  workspace_id,
  platform,
  provider_identity,
  reason,
  source_account_ids,
  source_profile_ids,
  source_external_user_ids,
  source_connection_types,
  details
)
SELECT
  workspace_id,
  platform,
  NULL,
  'missing_provider_identity',
  ARRAY[account_id],
  ARRAY[profile_id],
  CASE
    WHEN external_user_id IS NULL THEN ARRAY[]::TEXT[]
    ELSE ARRAY[external_user_id]
  END,
  ARRAY[connection_type],
  jsonb_build_object('account_id', account_id)
FROM source
WHERE provider_identity IS NULL
  AND connection_id IS NULL
  -- Already-disconnected rows are not publishable and nobody is waiting on
  -- them. Recording evidence for them only inflates the operator's conflict
  -- count with rows that need no action.
  AND is_live;

-- Classify every non-null identity group before creating any connection. The
-- expressions are intentionally explicit because these are security boundaries,
-- not best-effort data cleanup heuristics.
WITH source AS (
  SELECT
    sa.id AS account_id,
    sa.profile_id,
    sa.connection_id,
    p.workspace_id,
    sa.platform,
    sa.connection_type,
    sa.external_user_id,
    COALESCE(sa.x_app_mode, '') AS x_app_mode,
    sa.access_token,
    COALESCE(sa.refresh_token, '') AS refresh_token,
    sa.token_expires_at,
    TO_JSONB(ARRAY(
      SELECT DISTINCT item
      FROM UNNEST(COALESCE(sa.scope, ARRAY[]::TEXT[])) AS item
      ORDER BY item
    )) AS authority_scope,
    COALESCE(sa.metadata, '{}'::JSONB)
      - 'dismissed_at'
      - 'disconnect_notified_at'
      - 'reconnect_required_at' AS authority_metadata,
    -- A row the customer already disconnected carries frozen credentials,
    -- scopes, routing metadata, and owner by definition. Divergence against such
    -- a row says nothing about the live account, so the classification counters
    -- below describe live rows only. One-row-per-Profile (account_count vs
    -- profile_count) still counts every row: it guards the
    -- (profile_id, connection_id) unique index, not staleness.
    (sa.disconnected_at IS NULL AND sa.status <> 'disconnected') AS is_live,
    CASE
      WHEN sa.platform = 'instagram'
        THEN NULLIF(BTRIM(sa.metadata->>'instagram_webhook_user_id'), '')
      WHEN sa.platform <> 'instagram'
        THEN NULLIF(BTRIM(sa.external_account_id), '')
    END AS provider_identity
  FROM social_accounts sa
  JOIN profiles p ON p.id = sa.profile_id
), grouped AS (
  SELECT
    workspace_id,
    platform,
    provider_identity,
    COUNT(*) AS account_count,
    COUNT(*) FILTER (WHERE is_live) AS live_account_count,
    COUNT(DISTINCT profile_id) AS profile_count,
    -- Ownership is a security boundary, but it must describe the accounts that
    -- are connected today. A row the customer disconnected long ago carries a
    -- frozen owner, and letting it vote quarantined the live account sharing its
    -- provider identity: in production 18 of 22 conflict groups were a single
    -- live row outvoted by dead rows. These counters therefore describe live
    -- rows only, and the attach step binds live rows only, so a dead row never
    -- joins another user's connection.
    COUNT(DISTINCT connection_type) FILTER (WHERE is_live) AS connection_type_count,
    COUNT(DISTINCT external_user_id) FILTER (
      WHERE external_user_id IS NOT NULL AND is_live
    ) AS managed_owner_count,
    COUNT(*) FILTER (
      WHERE connection_type = 'managed' AND external_user_id IS NULL AND is_live
    ) AS missing_managed_owner_count,
    COUNT(*) FILTER (
      WHERE connection_type = 'byo' AND external_user_id IS NOT NULL AND is_live
    ) AS byo_owner_count,
    -- access_token/refresh_token are deliberately NOT compared. They are
    -- AES-256-GCM ciphertexts with a fresh random nonce per Encrypt call, so
    -- COUNT(DISTINCT ...) over them is > 1 for any multi-row group even when
    -- the underlying credentials are byte-identical. Comparing them quarantined
    -- healthy accounts. Divergent credentials still surface through
    -- token_expiry_count, and credential_rank picks a deterministic winner.
    COUNT(DISTINCT x_app_mode) FILTER (WHERE is_live) AS app_mode_count,
    COUNT(DISTINCT COALESCE(token_expires_at::TEXT, '')) FILTER (WHERE is_live) AS token_expiry_count,
    COUNT(DISTINCT authority_scope) FILTER (WHERE is_live) AS scope_count,
    COUNT(DISTINCT authority_metadata) FILTER (WHERE is_live) AS routing_metadata_count,
    BOOL_OR(connection_type = 'managed' AND is_live) AS has_managed,
    BOOL_OR(connection_type = 'byo' AND is_live) AS has_byo,
    BOOL_OR(connection_id IS NULL) AS has_legacy_candidate,
    ARRAY_AGG(account_id ORDER BY account_id) AS account_ids,
    ARRAY_AGG(profile_id ORDER BY account_id) AS profile_ids,
    ARRAY_AGG(DISTINCT external_user_id) FILTER (
      WHERE external_user_id IS NOT NULL
    ) AS external_user_ids,
    ARRAY_AGG(DISTINCT connection_type ORDER BY connection_type) AS connection_types,
    JSONB_AGG(
      JSONB_BUILD_OBJECT(
        'account_id', account_id,
        'access_token_sha256', ENCODE(SHA256(CONVERT_TO(access_token, 'UTF8')), 'hex'),
        'refresh_token_sha256', ENCODE(SHA256(CONVERT_TO(refresh_token, 'UTF8')), 'hex'),
        'token_expires_at', token_expires_at,
        'scope', authority_scope,
        'routing_metadata_sha256', ENCODE(
          SHA256(CONVERT_TO(authority_metadata::TEXT, 'UTF8')),
          'hex'
        )
      ) ORDER BY account_id
    ) AS authority_evidence
  FROM source
  WHERE provider_identity IS NOT NULL
  GROUP BY workspace_id, platform, provider_identity
), unsafe AS (
  SELECT
    *,
    CASE
      WHEN connection_type_count > 1 OR (has_managed AND has_byo)
        THEN 'mixed_ownership'
      WHEN has_managed AND managed_owner_count > 1
        THEN 'cross_managed_user'
      WHEN has_managed AND missing_managed_owner_count > 0
        THEN 'missing_managed_owner'
      WHEN has_byo AND byo_owner_count > 0
        THEN 'owner_on_byo_connection'
      WHEN account_count <> profile_count
        THEN 'duplicate_profile_binding'
      WHEN app_mode_count > 1
        THEN 'incompatible_app_mode'
      WHEN token_expiry_count > 1
        THEN 'incompatible_credential_state'
      WHEN scope_count > 1
        THEN 'incompatible_scope'
      WHEN routing_metadata_count > 1
        THEN 'incompatible_routing_metadata'
    END AS reason
  FROM grouped
)
INSERT INTO social_connection_migration_conflicts (
  workspace_id,
  platform,
  provider_identity,
  reason,
  source_account_ids,
  source_profile_ids,
  source_external_user_ids,
  source_connection_types,
  details
)
SELECT
  workspace_id,
  platform,
  provider_identity,
  reason,
  account_ids,
  profile_ids,
  COALESCE(external_user_ids, ARRAY[]::TEXT[]),
  connection_types,
  jsonb_build_object(
    'account_count', account_count,
    'profile_count', profile_count,
    'managed_owner_count', managed_owner_count,
    'app_mode_count', app_mode_count,
    'live_account_count', live_account_count,
    'token_expiry_count', token_expiry_count,
    'scope_count', scope_count,
    'routing_metadata_count', routing_metadata_count,
    'authority_evidence', authority_evidence,
    'promotion_blocked', TRUE
  )
FROM unsafe
WHERE reason IS NOT NULL
  AND has_legacy_candidate;

-- Fail closed for every row whose physical identity could not be promoted
-- safely. These rows keep their legacy credentials for operator-assisted
-- re-verification, but they must not remain publishable while connection_id is
-- NULL: otherwise each public account ID would look like an independent
-- physical account to validation and quota code.
--
-- 'reconnect_required' alone is what stops publishing —
-- socialAccountDisconnectedForPublish and the retry policy_admission gate both
-- treat it as unavailable. disconnected_at is deliberately NOT written: every
-- account listing filters on `disconnected_at IS NULL`, so setting it would
-- erase the account from the Dashboard and from GET /v1/accounts, leaving the
-- customer with no surface to launch the targeted reconnect that clears the
-- quarantine. Quarantine must be visible and self-serviceable, not silent.
--
-- Already-disconnected rows are skipped. They are not publishable, the customer
-- is not waiting on them, and relabelling them 'reconnect_required' would
-- resurrect dead accounts in the UI as if they needed action.
UPDATE social_accounts sa
SET status = 'reconnect_required',
    metadata = COALESCE(sa.metadata, '{}'::JSONB)
      || JSONB_BUILD_OBJECT('reconnect_required_at', NOW()::TEXT)
FROM social_connection_migration_conflicts conflict
WHERE sa.id = ANY(conflict.source_account_ids)
  AND sa.connection_id IS NULL
  AND sa.disconnected_at IS NULL
  AND sa.status <> 'disconnected';

-- Create one connection only for groups that passed every ownership, routing,
-- and stable-binding check. Credential precedence is deterministic: active,
-- latest verified refresh, latest connection, then public account ID.
WITH source AS (
  SELECT
    sa.*,
    p.workspace_id,
    (sa.disconnected_at IS NULL AND sa.status <> 'disconnected') AS is_live,
    CASE
      WHEN sa.platform = 'instagram'
        THEN NULLIF(BTRIM(sa.metadata->>'instagram_webhook_user_id'), '')
      WHEN sa.platform <> 'instagram'
        THEN NULLIF(BTRIM(sa.external_account_id), '')
    END AS provider_identity
  FROM social_accounts sa
  JOIN profiles p ON p.id = sa.profile_id
), grouped AS (
  SELECT
    workspace_id,
    platform,
    provider_identity,
    COUNT(*) AS account_count,
    COUNT(DISTINCT profile_id) AS profile_count,
    -- Ownership is a security boundary, but it must describe the accounts that
    -- are connected today. A row the customer disconnected long ago carries a
    -- frozen owner, and letting it vote quarantined the live account sharing its
    -- provider identity: in production 18 of 22 conflict groups were a single
    -- live row outvoted by dead rows. These counters therefore describe live
    -- rows only, and the attach step binds live rows only, so a dead row never
    -- joins another user's connection.
    COUNT(DISTINCT connection_type) FILTER (WHERE is_live) AS connection_type_count,
    COUNT(DISTINCT external_user_id) FILTER (
      WHERE external_user_id IS NOT NULL AND is_live
    ) AS managed_owner_count,
    COUNT(*) FILTER (
      WHERE connection_type = 'managed' AND external_user_id IS NULL AND is_live
    ) AS missing_managed_owner_count,
    COUNT(*) FILTER (
      WHERE connection_type = 'byo' AND external_user_id IS NOT NULL AND is_live
    ) AS byo_owner_count,
    -- Must stay identical to the classification pass above, or a group could be
    -- both quarantined and promoted. See there for why the encrypted token
    -- columns are not compared and why staleness counters are live-scoped.
    COUNT(DISTINCT COALESCE(x_app_mode, '')) FILTER (WHERE is_live) AS app_mode_count,
    COUNT(DISTINCT COALESCE(token_expires_at::TEXT, '')) FILTER (WHERE is_live) AS token_expiry_count,
    COUNT(DISTINCT TO_JSONB(ARRAY(
      SELECT DISTINCT item
      FROM UNNEST(COALESCE(scope, ARRAY[]::TEXT[])) AS item
      ORDER BY item
    ))) FILTER (WHERE is_live) AS scope_count,
    COUNT(DISTINCT (
      COALESCE(metadata, '{}'::JSONB)
        - 'dismissed_at'
        - 'disconnect_notified_at'
        - 'reconnect_required_at'
    )) FILTER (WHERE is_live) AS routing_metadata_count,
    BOOL_OR(connection_type = 'managed' AND is_live) AS has_managed,
    BOOL_OR(connection_type = 'byo' AND is_live) AS has_byo,
    BOOL_OR(connection_id IS NULL) AS has_legacy_candidate
  FROM source
  WHERE provider_identity IS NOT NULL
  GROUP BY workspace_id, platform, provider_identity
), eligible AS (
  SELECT *
  FROM grouped
  WHERE connection_type_count = 1
    AND has_legacy_candidate
    AND NOT (has_managed AND has_byo)
    AND (NOT has_managed OR (managed_owner_count = 1 AND missing_managed_owner_count = 0))
    AND (NOT has_byo OR byo_owner_count = 0)
    AND account_count = profile_count
    AND app_mode_count <= 1
    AND token_expiry_count <= 1
    AND scope_count <= 1
    AND routing_metadata_count <= 1
), ranked AS (
  SELECT
    source.*,
    ROW_NUMBER() OVER (
      PARTITION BY source.workspace_id, source.platform, source.provider_identity
      ORDER BY
        (source.status = 'active' AND source.disconnected_at IS NULL) DESC,
        source.last_refreshed_at DESC NULLS LAST,
        source.connected_at DESC,
        source.id
    ) AS credential_rank
  FROM source
  JOIN eligible
    ON eligible.workspace_id = source.workspace_id
   AND eligible.platform = source.platform
   AND eligible.provider_identity = source.provider_identity
)
INSERT INTO social_connections (
  workspace_id,
  platform,
  provider_identity,
  access_token,
  refresh_token,
  token_expires_at,
  account_name,
  account_avatar_url,
  metadata,
  scope,
  status,
  connection_type,
  external_user_id,
  external_user_email,
  last_refreshed_at,
  x_app_mode,
  connected_at,
  disconnected_at
)
SELECT
  workspace_id,
  platform,
  provider_identity,
  access_token,
  refresh_token,
  token_expires_at,
  account_name,
  account_avatar_url,
  COALESCE(metadata, '{}'::JSONB),
  COALESCE(scope, ARRAY[]::TEXT[]),
  CASE
    WHEN status IN ('active', 'reconnect_required', 'disconnected') THEN status
    WHEN disconnected_at IS NULL THEN 'active'
    ELSE 'disconnected'
  END,
  connection_type,
  external_user_id,
  external_user_email,
  last_refreshed_at,
  x_app_mode,
  connected_at,
  disconnected_at
FROM ranked
WHERE credential_rank = 1
ON CONFLICT (workspace_id, platform, provider_identity)
  WHERE provider_identity IS NOT NULL
    AND status <> 'migration_conflict'
DO NOTHING;

-- Record the exact winning public binding and non-secret evidence for every
-- promoted group before attaching bindings to the new authority.
WITH source AS (
  SELECT
    sa.*,
    p.workspace_id,
    (sa.disconnected_at IS NULL AND sa.status <> 'disconnected') AS is_live,
    CASE
      WHEN sa.platform = 'instagram'
        THEN NULLIF(BTRIM(sa.metadata->>'instagram_webhook_user_id'), '')
      WHEN sa.platform <> 'instagram'
        THEN NULLIF(BTRIM(sa.external_account_id), '')
    END AS provider_identity,
    TO_JSONB(ARRAY(
      SELECT DISTINCT item
      FROM UNNEST(COALESCE(sa.scope, ARRAY[]::TEXT[])) AS item
      ORDER BY item
    )) AS authority_scope,
    COALESCE(sa.metadata, '{}'::JSONB)
      - 'dismissed_at'
      - 'disconnect_notified_at'
      - 'reconnect_required_at' AS authority_metadata
  FROM social_accounts sa
  JOIN profiles p ON p.id = sa.profile_id
  WHERE sa.connection_id IS NULL
), ranked AS (
  SELECT
    source.*,
    sc.id AS promoted_connection_id,
    ROW_NUMBER() OVER (
      PARTITION BY source.workspace_id, source.platform, source.provider_identity
      ORDER BY
        (source.status = 'active' AND source.disconnected_at IS NULL) DESC,
        source.last_refreshed_at DESC NULLS LAST,
        source.connected_at DESC,
        source.id
    ) AS credential_rank
  FROM source
  JOIN social_connections sc
    ON sc.workspace_id = source.workspace_id
   AND sc.platform = source.platform
   AND sc.provider_identity = source.provider_identity
   AND sc.status <> 'migration_conflict'
  WHERE NOT EXISTS (
    SELECT 1
    FROM social_connection_migration_conflicts conflict
    WHERE source.id = ANY(conflict.source_account_ids)
  )
)
INSERT INTO social_connection_migration_audit (
  workspace_id,
  platform,
  provider_identity,
  connection_id,
  outcome,
  selected_source_account_id,
  source_account_ids,
  authority_evidence
)
SELECT
  workspace_id,
  platform,
  provider_identity,
  promoted_connection_id,
  'promoted',
  MAX(id) FILTER (WHERE credential_rank = 1),
  ARRAY_AGG(id ORDER BY id),
  JSONB_BUILD_OBJECT(
    'authority_rows', JSONB_AGG(
      JSONB_BUILD_OBJECT(
        'account_id', id,
        'access_token_sha256', ENCODE(SHA256(CONVERT_TO(access_token, 'UTF8')), 'hex'),
        'refresh_token_sha256', ENCODE(SHA256(CONVERT_TO(COALESCE(refresh_token, ''), 'UTF8')), 'hex'),
        'token_expires_at', token_expires_at,
        'scope', authority_scope,
        'routing_metadata_sha256', ENCODE(
          SHA256(CONVERT_TO(authority_metadata::TEXT, 'UTF8')),
          'hex'
        ),
        'selected', credential_rank = 1
      ) ORDER BY id
    )
  )
FROM ranked
GROUP BY workspace_id, platform, provider_identity, promoted_connection_id;

-- Only safe groups have a matching connection row, so conflict rows retain a
-- null connection_id and continue to use the old credential/owner columns.
--
-- The ownership equality below is what makes live-scoped classification safe.
-- Classification now ignores already-disconnected rows when deciding whether a
-- group is ambiguous, so a group can contain a dead row whose owner differs
-- from the promoted connection. Such a row must not be attached: in a managed
-- workspace that would bind one end user's historical row to another end user's
-- credentials. It keeps connection_id NULL instead, which leaves it exactly as
-- it is today — disconnected and unpublishable.
--
-- Live rows always satisfy this equality by construction: their group passed
-- the live ownership checks, and the promoted connection is built from the
-- highest-ranked live row in that same group.
UPDATE social_accounts sa
SET connection_id = sc.id
FROM profiles p, social_connections sc
WHERE p.id = sa.profile_id
  AND sa.connection_id IS NULL
  AND sc.workspace_id = p.workspace_id
  AND sc.platform = sa.platform
  AND sc.provider_identity = CASE
    WHEN sa.platform = 'instagram'
      THEN NULLIF(BTRIM(sa.metadata->>'instagram_webhook_user_id'), '')
    WHEN sa.platform <> 'instagram'
      THEN NULLIF(BTRIM(sa.external_account_id), '')
  END
  AND sa.connection_type = sc.connection_type
  AND sa.external_user_id IS NOT DISTINCT FROM sc.external_user_id
  AND NOT EXISTS (
    SELECT 1
    FROM social_connection_migration_conflicts conflict
    WHERE sa.id = ANY(conflict.source_account_ids)
  );

-- Pending work must resolve to a safe, versioned binding or cutover aborts.
UPDATE post_delivery_jobs job
SET connection_id = account.connection_id,
    binding_version = account.binding_version,
    updated_at = NOW()
FROM social_accounts account
WHERE account.id = job.social_account_id
  AND job.state = 'pending'
  AND account.connection_id IS NOT NULL;

DO $$
BEGIN
  IF EXISTS (
    SELECT 1
    FROM post_delivery_jobs job
    JOIN social_accounts account ON account.id = job.social_account_id
    WHERE job.state = 'pending'
      AND (
        account.connection_id IS NULL
        OR job.connection_id IS DISTINCT FROM account.connection_id
        OR job.binding_version IS DISTINCT FROM account.binding_version
      )
  ) THEN
    RAISE EXCEPTION 'pending delivery job cannot be reconciled to connection authority';
  END IF;
END;
$$;

-- Detect historical sibling selections before collapsing legacy target keys.
DO $$
BEGIN
  IF EXISTS (
    SELECT 1
    FROM social_post_physical_targets target
    JOIN social_accounts account ON account.id = target.selected_social_account_id
    JOIN social_post_physical_targets sibling
      ON sibling.post_id = target.post_id
     AND sibling.physical_target_key <> target.physical_target_key
    JOIN social_accounts sibling_account ON sibling_account.id = sibling.selected_social_account_id
    WHERE target.physical_target_key LIKE 'legacy-account:%'
      AND sibling.physical_target_key LIKE 'legacy-account:%'
      AND account.connection_id IS NOT NULL
      AND account.connection_id = sibling_account.connection_id
      AND account.id <> sibling_account.id
      AND EXISTS (
        SELECT 1 FROM post_delivery_jobs job
        WHERE job.post_id = target.post_id
          AND job.social_account_id IN (account.id, sibling_account.id)
          AND job.state IN ('pending', 'running', 'retrying')
      )
  ) THEN
    RAISE EXCEPTION 'nonterminal sibling physical targets cannot be collapsed safely';
  END IF;
END;
$$;

WITH legacy AS (
  SELECT
    target.post_id,
    account.connection_id,
    MIN(target.selected_social_account_id) AS selected_social_account_id,
    COUNT(DISTINCT target.selected_social_account_id) AS binding_count,
    JSONB_BUILD_OBJECT(
      'source_target_keys', ARRAY_AGG(target.physical_target_key ORDER BY target.physical_target_key),
      'source_account_ids', ARRAY_AGG(DISTINCT target.selected_social_account_id ORDER BY target.selected_social_account_id)
    ) AS details
  FROM social_post_physical_targets target
  JOIN social_accounts account ON account.id = target.selected_social_account_id
  WHERE target.physical_target_key LIKE 'legacy-account:%'
    AND account.connection_id IS NOT NULL
  GROUP BY target.post_id, account.connection_id
)
INSERT INTO social_post_physical_targets (
  post_id, physical_target_key, connection_id, selected_social_account_id, status, conflict_details
)
SELECT
  post_id,
  'connection:' || connection_id,
  connection_id,
  selected_social_account_id,
  CASE WHEN binding_count = 1 THEN 'active' ELSE 'migration_conflict' END,
  CASE WHEN binding_count = 1 THEN '{}'::JSONB ELSE details END
FROM legacy
ON CONFLICT (post_id, physical_target_key) DO UPDATE SET
  status = CASE
    WHEN social_post_physical_targets.selected_social_account_id = EXCLUDED.selected_social_account_id
      AND social_post_physical_targets.status = 'active'
      AND EXCLUDED.status = 'active'
    THEN 'active'
    ELSE 'migration_conflict'
  END,
  conflict_details = social_post_physical_targets.conflict_details || EXCLUDED.conflict_details,
  updated_at = NOW();

DELETE FROM social_post_physical_targets target
USING social_accounts account
WHERE target.selected_social_account_id = account.id
  AND target.physical_target_key LIKE 'legacy-account:%'
  AND account.connection_id IS NOT NULL;

-- Preserve duplicate Inbox evidence while assigning one canonical connection row.
WITH ranked AS (
  SELECT
    item.id,
    account.connection_id,
    FIRST_VALUE(item.id) OVER (
      PARTITION BY account.connection_id, item.source, item.external_id
      ORDER BY item.received_at, item.created_at, item.id
    ) AS canonical_id,
    ROW_NUMBER() OVER (
      PARTITION BY account.connection_id, item.source, item.external_id
      ORDER BY item.received_at, item.created_at, item.id
    ) AS duplicate_rank
  FROM inbox_items item
  JOIN social_accounts account ON account.id = item.social_account_id
  WHERE account.connection_id IS NOT NULL
)
INSERT INTO inbox_item_supersessions (inbox_item_id, canonical_inbox_item_id)
SELECT id, canonical_id FROM ranked WHERE duplicate_rank > 1
ON CONFLICT (inbox_item_id) DO NOTHING;

WITH ranked AS (
  SELECT
    item.id,
    account.connection_id,
    ROW_NUMBER() OVER (
      PARTITION BY account.connection_id, item.source, item.external_id
      ORDER BY item.received_at, item.created_at, item.id
    ) AS duplicate_rank
  FROM inbox_items item
  JOIN social_accounts account ON account.id = item.social_account_id
  WHERE account.connection_id IS NOT NULL
)
UPDATE inbox_items item
SET connection_id = ranked.connection_id
FROM ranked
WHERE ranked.id = item.id AND ranked.duplicate_rank = 1;

ALTER TABLE inbox_items
  DROP CONSTRAINT IF EXISTS inbox_items_social_account_id_external_id_key;
CREATE UNIQUE INDEX IF NOT EXISTS inbox_items_legacy_account_source_external_unique_idx
  ON inbox_items (social_account_id, source, external_id)
  WHERE connection_id IS NULL;

-- Rewrite durable daily operation keys, then conserve reservation units exactly.
UPDATE physical_daily_publish_operations operation
SET physical_account_id = account.connection_id,
    updated_at = NOW()
FROM social_accounts account
WHERE operation.physical_account_id = account.id
  AND account.connection_id IS NOT NULL;

WITH legacy AS (
  SELECT
    reservation.workspace_id,
    account.connection_id AS physical_account_id,
    reservation.platform,
    reservation.utc_date,
    SUM(reservation.reserved_count)::INTEGER AS reserved_count
  FROM physical_daily_publish_reservations reservation
  JOIN social_accounts account ON account.id = reservation.physical_account_id
  WHERE account.connection_id IS NOT NULL
  GROUP BY reservation.workspace_id, account.connection_id, reservation.platform, reservation.utc_date
), deleted AS (
  DELETE FROM physical_daily_publish_reservations reservation
  USING social_accounts account
  WHERE reservation.physical_account_id = account.id
    AND account.connection_id IS NOT NULL
  RETURNING 1
)
INSERT INTO physical_daily_publish_reservations (
  workspace_id, physical_account_id, platform, utc_date, reserved_count
)
SELECT workspace_id, physical_account_id, platform, utc_date, reserved_count
FROM legacy
ON CONFLICT (workspace_id, physical_account_id, platform, utc_date) DO UPDATE SET
  reserved_count = physical_daily_publish_reservations.reserved_count + EXCLUDED.reserved_count,
  updated_at = NOW();

DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM social_accounts
    WHERE connection_id IS NULL
      AND binding_status = 'active'
      AND status = 'active'
      AND disconnected_at IS NULL
  ) THEN
    RAISE EXCEPTION 'cutover postcondition failed: publishable null binding';
  END IF;
  IF EXISTS (
    SELECT 1
    FROM social_accounts account
    JOIN profiles profile ON profile.id = account.profile_id
    JOIN social_connections connection ON connection.id = account.connection_id
    WHERE connection.workspace_id <> profile.workspace_id
       OR connection.platform <> account.platform
  ) THEN
    RAISE EXCEPTION 'cutover postcondition failed: connection scope mismatch';
  END IF;
  IF EXISTS (
    SELECT 1 FROM post_delivery_jobs
    WHERE state = 'pending' AND (connection_id IS NULL OR binding_version IS NULL)
  ) THEN
    RAISE EXCEPTION 'cutover postcondition failed: pending job snapshot missing';
  END IF;
END;
$$;

UPDATE social_connection_rollout_state
SET phase = 'cutover',
    cutover_application_sha = current_setting('unipost.cutover_sha', TRUE),
    cutover_environment_id = current_setting('unipost.cutover_environment_id', TRUE),
    cutover_completed_at = NOW(),
    cutover_backend_pid = NULL,
    updated_at = NOW()
WHERE id = 'global' AND phase = 'cutting_over';

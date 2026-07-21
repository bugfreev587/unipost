\set ON_ERROR_STOP on
BEGIN;
SET TRANSACTION READ ONLY;

SELECT COUNT(*) AS active_creatorless_api_keys
FROM api_keys
WHERE BTRIM(created_by_user_id) = ''
  AND revoked_at IS NULL
  AND (expires_at IS NULL OR expires_at > NOW());

WITH provider_ownership AS (
  SELECT
    p.workspace_id,
    sa.platform,
    CASE
      WHEN sa.platform = 'instagram'
        THEN NULLIF(sa.metadata->>'instagram_webhook_user_id', '')
      ELSE NULLIF(sa.external_account_id, '')
    END AS provider_identity,
    COUNT(*) AS active_rows,
    COUNT(DISTINCT COALESCE(sa.external_user_id, '__owner_byo__')) AS owner_count
  FROM social_accounts sa
  JOIN profiles p ON p.id = sa.profile_id
  WHERE sa.status = 'active'
    AND sa.disconnected_at IS NULL
  GROUP BY p.workspace_id, sa.platform, provider_identity
)
SELECT
  workspace_id,
  platform,
  MD5(provider_identity) AS provider_identity_hash,
  active_rows,
  owner_count
FROM provider_ownership
WHERE provider_identity IS NOT NULL
  AND (active_rows > 1 OR owner_count > 1)
ORDER BY workspace_id, platform, provider_identity;

ROLLBACK;

-- name: ListManagedUsersByProfile :many
SELECT
  external_user_id::TEXT AS external_user_id,
  COALESCE(
    MAX(external_user_email) FILTER (WHERE external_user_email IS NOT NULL AND external_user_email <> ''),
    ''
  )::TEXT AS external_user_email,
  COUNT(*)::INTEGER AS account_count,
  COUNT(*) FILTER (WHERE platform = 'twitter')::INTEGER  AS twitter_count,
  COUNT(*) FILTER (WHERE platform = 'linkedin')::INTEGER AS linkedin_count,
  COUNT(*) FILTER (WHERE platform = 'bluesky')::INTEGER  AS bluesky_count,
  COUNT(*) FILTER (WHERE platform = 'youtube')::INTEGER  AS youtube_count,
  COUNT(*) FILTER (WHERE status = 'reconnect_required')::INTEGER AS reconnect_count,
  COUNT(*) FILTER (WHERE disconnected_at IS NOT NULL OR status = 'disconnected')::INTEGER AS disconnected_count,
  MIN(connected_at)::TIMESTAMPTZ   AS first_connected_at,
  MAX(last_refreshed_at)::TIMESTAMPTZ AS last_refreshed_at
FROM social_accounts
WHERE profile_id = $1
  AND external_user_id IS NOT NULL
  AND COALESCE(metadata->>'dismissed_at', '') = ''
  AND connection_type = 'managed'
GROUP BY external_user_id
ORDER BY MIN(connected_at) DESC, external_user_id DESC
LIMIT $2;

-- name: ListManagedAccountsByExternalUser :many
SELECT id, profile_id, platform, access_token, refresh_token, token_expires_at,
  external_account_id, account_name, account_avatar_url, connected_at,
  disconnected_at, metadata, scope, status, connection_type, connect_session_id,
  external_user_id, external_user_email, last_refreshed_at
FROM social_accounts
WHERE profile_id = $1
  AND external_user_id = $2
  AND COALESCE(metadata->>'dismissed_at', '') = ''
  AND connection_type = 'managed'
ORDER BY connected_at DESC;

-- name: CountManagedUsersByProfile :one
SELECT COUNT(DISTINCT external_user_id)::INTEGER AS total
FROM social_accounts
WHERE profile_id = $1
  AND external_user_id IS NOT NULL
  AND COALESCE(metadata->>'dismissed_at', '') = ''
  AND connection_type = 'managed';

-- name: DismissDisconnectedManagedAccountsByExternalUser :execrows
UPDATE social_accounts
SET metadata = COALESCE(metadata, '{}'::jsonb) || jsonb_build_object('dismissed_at', NOW()::TEXT)
WHERE profile_id = $1
  AND external_user_id = $2
  AND connection_type = 'managed'
  AND (disconnected_at IS NOT NULL OR status = 'disconnected')
  AND COALESCE(metadata->>'dismissed_at', '') = '';

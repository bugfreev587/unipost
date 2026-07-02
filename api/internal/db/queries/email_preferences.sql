-- name: ListEmailPreferencesByUser :many
SELECT *
FROM email_preferences
WHERE user_id = $1
ORDER BY category_key ASC;

-- name: GetEmailPreferenceForSend :one
SELECT *
FROM email_preferences
WHERE user_id = sqlc.arg(user_id)
  AND category_key = sqlc.arg(category_key)
LIMIT 1;

-- name: UpsertEmailPreference :one
INSERT INTO email_preferences (
  user_id,
  email,
  category_key,
  enabled,
  source
)
VALUES (
  sqlc.arg(user_id),
  sqlc.arg(email),
  sqlc.arg(category_key),
  sqlc.arg(enabled),
  sqlc.arg(source)
)
ON CONFLICT (user_id, category_key)
DO UPDATE SET
  email = EXCLUDED.email,
  enabled = EXCLUDED.enabled,
  source = EXCLUDED.source,
  updated_at = NOW()
RETURNING *;

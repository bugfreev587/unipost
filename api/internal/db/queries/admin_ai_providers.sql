-- name: GetAdminAIProviderKey :one
SELECT * FROM admin_ai_provider_keys
WHERE provider = $1;

-- name: ListAdminAIProviderKeys :many
SELECT * FROM admin_ai_provider_keys
ORDER BY provider;

-- name: UpsertAdminAIProviderKey :one
INSERT INTO admin_ai_provider_keys (
    provider,
    enabled,
    api_key_ciphertext,
    key_tail,
    base_url,
    chat_model,
    messages_model,
    last_rotated_at,
    created_by_admin_id,
    updated_by_admin_id
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
ON CONFLICT (provider) DO UPDATE
SET enabled = EXCLUDED.enabled,
    api_key_ciphertext = EXCLUDED.api_key_ciphertext,
    key_tail = EXCLUDED.key_tail,
    base_url = EXCLUDED.base_url,
    chat_model = EXCLUDED.chat_model,
    messages_model = EXCLUDED.messages_model,
    last_rotated_at = EXCLUDED.last_rotated_at,
    updated_by_admin_id = EXCLUDED.updated_by_admin_id,
    updated_at = NOW()
RETURNING *;

-- name: UpdateAdminAIProviderConfig :one
UPDATE admin_ai_provider_keys
SET enabled = $2,
    base_url = $3,
    chat_model = $4,
    messages_model = $5,
    updated_by_admin_id = $6,
    updated_at = NOW()
WHERE provider = $1
RETURNING *;

-- name: UpdateAdminAIProviderValidation :one
UPDATE admin_ai_provider_keys
SET last_validated_at = NOW(),
    last_validation_status = $2,
    last_validation_error = $3,
    updated_by_admin_id = $4,
    updated_at = NOW()
WHERE provider = $1
RETURNING *;

-- name: DisableAdminAIProviderKey :one
UPDATE admin_ai_provider_keys
SET enabled = false,
    updated_by_admin_id = $2,
    updated_at = NOW()
WHERE provider = $1
RETURNING *;

-- name: DeleteAISurfaceRoutesForProvider :exec
DELETE FROM ai_surface_routing
WHERE provider = $1;

-- name: GetAISurfaceRoute :one
SELECT * FROM ai_surface_routing
WHERE surface = $1;

-- name: ListAISurfaceRoutes :many
SELECT * FROM ai_surface_routing
ORDER BY surface;

-- name: UpsertAISurfaceRoute :one
INSERT INTO ai_surface_routing (
    surface,
    provider,
    client_kind,
    model_override,
    created_by_admin_id,
    updated_by_admin_id
)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (surface) DO UPDATE
SET provider = EXCLUDED.provider,
    client_kind = EXCLUDED.client_kind,
    model_override = EXCLUDED.model_override,
    updated_by_admin_id = EXCLUDED.updated_by_admin_id,
    updated_at = NOW()
RETURNING *;

-- name: DeleteAISurfaceRoute :exec
DELETE FROM ai_surface_routing
WHERE surface = $1;

-- name: CreateAdminAIProviderEvent :one
INSERT INTO admin_ai_provider_events (
    provider,
    surface,
    action,
    category,
    actor_admin_id,
    metadata
)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListAdminAIProviderEvents :many
SELECT * FROM admin_ai_provider_events
WHERE (sqlc.arg(provider_filter)::TEXT = '' OR provider = sqlc.arg(provider_filter))
  AND (sqlc.arg(action_filter)::TEXT = '' OR action = sqlc.arg(action_filter))
  AND (sqlc.arg(before_id)::BIGINT = 0 OR id < sqlc.arg(before_id))
ORDER BY id DESC
LIMIT sqlc.arg(limit_rows);

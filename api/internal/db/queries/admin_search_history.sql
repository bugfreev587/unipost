-- name: ListAdminSearchHistory :many
SELECT * FROM admin_search_history
WHERE admin_user_id = @admin_user_id
  AND field_key = @field_key
ORDER BY last_used_at DESC, usage_count DESC
LIMIT @limit_rows;

-- name: UpsertAdminSearchHistory :one
INSERT INTO admin_search_history (
    admin_user_id,
    field_key,
    value,
    value_normalized
)
VALUES (
    @admin_user_id,
    @field_key,
    @value,
    @value_normalized
)
ON CONFLICT (admin_user_id, field_key, value_normalized) DO UPDATE
SET value = EXCLUDED.value,
    usage_count = admin_search_history.usage_count + 1,
    last_used_at = NOW(),
    updated_at = NOW()
RETURNING *;

-- name: DeleteAdminSearchHistory :execrows
DELETE FROM admin_search_history
WHERE id = @id
  AND admin_user_id = @admin_user_id;

-- name: PruneAdminSearchHistory :execrows
DELETE FROM admin_search_history AS ash
WHERE id IN (
    SELECT inner_ash.id
    FROM admin_search_history AS inner_ash
    WHERE inner_ash.admin_user_id = @admin_user_id
      AND inner_ash.field_key = @field_key
    ORDER BY inner_ash.last_used_at DESC, inner_ash.usage_count DESC, inner_ash.created_at DESC
    OFFSET @keep_rows
);

-- name: CleanupExpiredAdminSearchHistory :execrows
DELETE FROM admin_search_history
WHERE last_used_at < NOW() - INTERVAL '180 days';

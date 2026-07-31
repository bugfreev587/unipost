-- name: ReserveSocialPostPhysicalTargets :exec
SELECT reserve_social_post_physical_targets(
  sqlc.arg(post_id)::TEXT,
  sqlc.arg(account_ids)::TEXT[],
  sqlc.arg(connection_ids)::TEXT[]
);

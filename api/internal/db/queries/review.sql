-- name: CreateReviewDomain :one
INSERT INTO review_domains (
  workspace_id, domain, provider, status, verification_token, cname_target, tls_status
) VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetReviewDomain :one
SELECT * FROM review_domains
WHERE id = $1 AND workspace_id = $2;

-- name: GetReviewDomainByDomain :one
SELECT * FROM review_domains
WHERE lower(domain) = lower($1);

-- name: ListReviewDomainsByWorkspace :many
SELECT * FROM review_domains
WHERE workspace_id = $1
ORDER BY created_at DESC;

-- name: UpdateReviewDomainVerification :one
UPDATE review_domains
SET status = $3,
    dns_verified_at = $4,
    tls_status = $5,
    tls_issued_at = $6,
    updated_at = NOW()
WHERE id = $1 AND workspace_id = $2
RETURNING *;

-- name: CreateReviewKit :one
INSERT INTO review_kits (
  workspace_id, platform, use_case, review_domain_id, brand_snapshot, required_scopes, status
) VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetReviewKit :one
SELECT * FROM review_kits
WHERE id = $1 AND workspace_id = $2;

-- name: ListReviewKitsByWorkspace :many
SELECT * FROM review_kits
WHERE workspace_id = $1
ORDER BY created_at DESC;

-- name: UpdateReviewKitStatus :one
UPDATE review_kits
SET status = $3,
    updated_at = NOW()
WHERE id = $1 AND workspace_id = $2
RETURNING *;

-- name: CreateReviewJob :one
INSERT INTO review_jobs (
  review_kit_id, workspace_id, platform, status, agent_version, review_session_token_id
) VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetReviewJob :one
SELECT * FROM review_jobs
WHERE id = $1 AND workspace_id = $2;

-- name: ListReviewJobsByKit :many
SELECT * FROM review_jobs
WHERE review_kit_id = $1 AND workspace_id = $2
ORDER BY created_at DESC;

-- name: MarkReviewJobRunning :one
UPDATE review_jobs
SET status = 'running',
    started_at = COALESCE(started_at, NOW()),
    updated_at = NOW()
WHERE id = $1 AND workspace_id = $2
RETURNING *;

-- name: MarkReviewJobWaitingForUser :one
UPDATE review_jobs
SET status = 'waiting_for_user',
    updated_at = NOW()
WHERE id = $1 AND workspace_id = $2
RETURNING *;

-- name: CompleteReviewJob :one
UPDATE review_jobs
SET status = 'completed',
    completed_at = NOW(),
    video_file_id = $3,
    artifacts_json = $4,
    updated_at = NOW()
WHERE id = $1 AND workspace_id = $2
RETURNING *;

-- name: FailReviewJob :one
UPDATE review_jobs
SET status = 'failed',
    failed_at = NOW(),
    failure_reason = $3,
    artifacts_json = $4,
    updated_at = NOW()
WHERE id = $1 AND workspace_id = $2
RETURNING *;

-- name: AttachReviewSessionToJob :one
UPDATE review_jobs
SET review_session_token_id = $3,
    updated_at = NOW()
WHERE id = $1 AND workspace_id = $2
RETURNING *;

-- name: CreateReviewJobEvent :one
INSERT INTO review_job_events (
  review_job_id, event_type, message, metadata, elapsed_ms
) VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListReviewJobEvents :many
SELECT e.* FROM review_job_events e
JOIN review_jobs j ON j.id = e.review_job_id
WHERE e.review_job_id = $1 AND j.workspace_id = $2
ORDER BY e.created_at ASC;

-- name: CreateReviewSession :one
INSERT INTO review_sessions (
  review_job_id, review_kit_id, workspace_id, platform, review_domain, token_hash, expires_at
) VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetReviewSessionByTokenHash :one
SELECT * FROM review_sessions
WHERE token_hash = $1
  AND revoked_at IS NULL
  AND expires_at > NOW();

-- name: ClaimReviewSession :one
UPDATE review_sessions
SET claimed_at = COALESCE(claimed_at, NOW())
WHERE id = $1
  AND revoked_at IS NULL
  AND expires_at > NOW()
RETURNING *;

-- name: RevokeReviewSession :one
UPDATE review_sessions
SET revoked_at = NOW()
WHERE id = $1 AND workspace_id = $2
RETURNING *;

-- name: GetActiveReviewSessionForJob :one
SELECT * FROM review_sessions
WHERE review_job_id = $1
  AND workspace_id = $2
  AND revoked_at IS NULL
  AND expires_at > NOW()
ORDER BY created_at DESC
LIMIT 1;

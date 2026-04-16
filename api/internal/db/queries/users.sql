-- name: UpsertUser :one
INSERT INTO users (id, email, name)
VALUES ($1, $2, $3)
ON CONFLICT (id)
DO UPDATE SET email = $2, name = $3, updated_at = NOW()
RETURNING *;

-- name: GetUser :one
SELECT * FROM users WHERE id = $1;

-- name: DeleteUser :exec
DELETE FROM users WHERE id = $1;

-- name: SetUserDefaultProfile :exec
UPDATE users SET default_profile_id = $2, updated_at = NOW()
WHERE id = $1;

-- name: SetUserLastProfile :exec
UPDATE users SET last_profile_id = $2 WHERE id = $1;

-- name: CompleteOnboarding :exec
UPDATE users SET onboarding_completed = TRUE, name = $2, updated_at = NOW()
WHERE id = $1;

-- name: MarkOnboardingShown :exec
-- Stamps the first-seen timestamp when the Welcome modal is rendered.
-- Safe to call repeatedly — only sets if currently NULL.
UPDATE users
SET onboarding_shown_at = COALESCE(onboarding_shown_at, NOW())
WHERE id = $1;

-- name: SetOnboardingIntent :one
-- Records the user's intent (or "skipped") and stamps the completion time.
-- Always updates (no COALESCE) so users can change their intent via Settings.
UPDATE users
SET onboarding_intent = $2,
    onboarding_completed_at = NOW(),
    onboarding_shown_at = COALESCE(onboarding_shown_at, NOW()),
    updated_at = NOW()
WHERE id = $1
RETURNING *;

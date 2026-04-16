-- name: ListUserTutorials :many
-- Returns every tutorial row for a user. The handler merges this with
-- activation counts so the frontend gets per-tutorial completion +
-- per-step completion in one response.
SELECT user_id, tutorial_id, completed_at, dismissed_at, created_at, updated_at
FROM user_tutorials
WHERE user_id = $1
ORDER BY created_at;

-- name: GetUserTutorial :one
SELECT user_id, tutorial_id, completed_at, dismissed_at, created_at, updated_at
FROM user_tutorials
WHERE user_id = $1 AND tutorial_id = $2;

-- name: CompleteUserTutorial :one
-- Stamps completed_at. If the row exists, keeps the earliest
-- completed_at (idempotent: re-completing doesn't reset the timestamp).
INSERT INTO user_tutorials (user_id, tutorial_id, completed_at, updated_at)
VALUES ($1, $2, NOW(), NOW())
ON CONFLICT (user_id, tutorial_id) DO UPDATE SET
  completed_at = COALESCE(user_tutorials.completed_at, EXCLUDED.completed_at),
  dismissed_at = NULL,  -- completing a tutorial clears any prior dismissal
  updated_at   = NOW()
RETURNING user_id, tutorial_id, completed_at, dismissed_at, created_at, updated_at;

-- name: DismissUserTutorial :one
-- Stamps dismissed_at. Tutorial-specific policy (e.g. quickstart being
-- re-popped on the profile page) lives in the frontend; the backend
-- just records the fact.
INSERT INTO user_tutorials (user_id, tutorial_id, dismissed_at, updated_at)
VALUES ($1, $2, NOW(), NOW())
ON CONFLICT (user_id, tutorial_id) DO UPDATE SET
  dismissed_at = NOW(),
  updated_at   = NOW()
RETURNING user_id, tutorial_id, completed_at, dismissed_at, created_at, updated_at;

-- name: ClearUserTutorialDismissal :exec
-- Clears dismissed_at so a mandatory tutorial can re-pop. Called when
-- the frontend decides to resume a mandatory tutorial on the profile
-- page. Does not touch completed_at.
UPDATE user_tutorials
SET dismissed_at = NULL, updated_at = NOW()
WHERE user_id = $1 AND tutorial_id = $2 AND completed_at IS NULL;

-- +goose Up

-- user_tutorials tracks per-tutorial completion and dismissal state so
-- we can support multiple tutorials (Quickstart, post_with_api, ...)
-- without adding a new column per tutorial to the users table.
--
-- Step completion within a tutorial is still derived from real counts
-- (see GetUserActivationCounts). This table only persists the two
-- lifecycle bits that need to survive a full page reload:
--   - completed_at: stamped once, used to hide the modal and decide
--     whether to show the celebration screen on next visit.
--   - dismissed_at: stamped when the user explicitly closes the modal
--     AFTER completing at least one step (see modal logic).
--
-- For mandatory tutorials (quickstart), dismissed_at hides the modal
-- temporarily — visiting the profile page re-pops it (resume from
-- the first incomplete step). For optional tutorials, dismissed_at
-- hides it permanently until the user explicitly re-opens from the
-- /tutorials page.
CREATE TABLE IF NOT EXISTS user_tutorials (
  user_id       TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  tutorial_id   TEXT NOT NULL,
  completed_at  TIMESTAMPTZ,
  dismissed_at  TIMESTAMPTZ,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (user_id, tutorial_id)
);

-- Backfill: users who already completed the old activation guide have
-- completed the quickstart. We drop the old create_api_key step from
-- quickstart as part of this refactor, so anyone whose activation was
-- marked complete already satisfies the new 2-step definition.
INSERT INTO user_tutorials (user_id, tutorial_id, completed_at, created_at, updated_at)
SELECT id, 'quickstart', activation_completed_at, activation_completed_at, activation_completed_at
FROM users
WHERE activation_completed_at IS NOT NULL
ON CONFLICT (user_id, tutorial_id) DO NOTHING;

-- Backfill dismissals too — if a user dismissed the old guide we
-- preserve that so they don't get re-popped immediately on rollout.
INSERT INTO user_tutorials (user_id, tutorial_id, dismissed_at, created_at, updated_at)
SELECT id, 'quickstart', activation_guide_dismissed_at, activation_guide_dismissed_at, activation_guide_dismissed_at
FROM users
WHERE activation_guide_dismissed_at IS NOT NULL
  AND activation_completed_at IS NULL
ON CONFLICT (user_id, tutorial_id) DO UPDATE SET
  dismissed_at = EXCLUDED.dismissed_at;

-- +goose Down

DROP TABLE IF EXISTS user_tutorials;

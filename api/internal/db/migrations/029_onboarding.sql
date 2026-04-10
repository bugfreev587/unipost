-- +goose Up
--
-- Onboarding flow: track whether a user has completed the post-signup
-- onboarding and store their selected usage modes on the workspace.
--
-- IF NOT EXISTS guards because this migration was originally numbered
-- 028 and may have partially applied before being renumbered to 029.

ALTER TABLE users
  ADD COLUMN IF NOT EXISTS onboarding_completed BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE workspaces
  ADD COLUMN IF NOT EXISTS usage_modes TEXT[] NOT NULL DEFAULT '{}';

UPDATE users SET onboarding_completed = TRUE WHERE onboarding_completed = FALSE;

-- +goose Down
ALTER TABLE workspaces DROP COLUMN IF EXISTS usage_modes;
ALTER TABLE users DROP COLUMN IF EXISTS onboarding_completed;

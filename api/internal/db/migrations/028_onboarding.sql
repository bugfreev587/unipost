-- +goose Up
--
-- Onboarding flow: track whether a user has completed the post-signup
-- onboarding and store their selected usage modes on the workspace.
--
-- usage_modes is a TEXT[] with values from {"personal", "whitelabel", "api"}.
-- An empty array means "show all features" (backward compatible).
--
-- Existing users are marked as onboarding-completed so they skip the flow.

ALTER TABLE users
  ADD COLUMN onboarding_completed BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE workspaces
  ADD COLUMN usage_modes TEXT[] NOT NULL DEFAULT '{}';

-- Backfill: existing users have already been using the dashboard,
-- so they don't need onboarding.
UPDATE users SET onboarding_completed = TRUE;

-- +goose Down
ALTER TABLE workspaces DROP COLUMN IF EXISTS usage_modes;
ALTER TABLE users DROP COLUMN IF EXISTS onboarding_completed;

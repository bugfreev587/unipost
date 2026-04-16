-- +goose Up

-- Activation-guide (empty state) companion to the onboarding-intent redesign.
-- Step completion is derived from real data (accounts, posts, api_keys) so
-- we only need two nullable timestamps on users:
--   - activation_completed_at: set once all 3 steps meet their thresholds
--   - activation_guide_dismissed_at: set when the user explicitly hides the card

ALTER TABLE users
  ADD COLUMN activation_completed_at TIMESTAMPTZ,
  ADD COLUMN activation_guide_dismissed_at TIMESTAMPTZ;

-- +goose Down

ALTER TABLE users
  DROP COLUMN activation_completed_at,
  DROP COLUMN activation_guide_dismissed_at;

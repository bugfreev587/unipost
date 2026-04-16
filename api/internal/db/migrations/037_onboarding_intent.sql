-- +goose Up

-- Intent-collection redesign: the old boolean onboarding_completed + usage_modes
-- gated dashboard access. This replaces it with a lightweight, skippable intent
-- question that never blocks access.
--
-- Values for onboarding_intent:
--   'exploring'     - just checking UniPost out
--   'own_accounts'  - publishing to own social accounts
--   'building_api'  - building a product on UniPost API
--   'skipped'       - user dismissed the modal without selecting
--   NULL            - modal never shown yet (will show on next dashboard load)

ALTER TABLE users
  ADD COLUMN onboarding_intent TEXT,
  ADD COLUMN onboarding_shown_at TIMESTAMPTZ,
  ADD COLUMN onboarding_completed_at TIMESTAMPTZ;

-- +goose Down

ALTER TABLE users
  DROP COLUMN onboarding_intent,
  DROP COLUMN onboarding_shown_at,
  DROP COLUMN onboarding_completed_at;

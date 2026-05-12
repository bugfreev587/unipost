-- +goose Up
--
-- White-label packaging refresh (May 2026):
-- Basic unlocks one-platform white-label, while Growth+ can optionally
-- remove the hosted Connect attribution footer.

ALTER TABLE profiles
  ADD COLUMN branding_hide_powered_by BOOLEAN NOT NULL DEFAULT FALSE;

-- +goose Down
ALTER TABLE profiles
  DROP COLUMN IF EXISTS branding_hide_powered_by;

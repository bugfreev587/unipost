-- +goose Up
--
-- R2-backed hosted Connect logo uploads. External logo URLs remain
-- represented by branding_logo_url with this key left NULL.

ALTER TABLE profiles
  ADD COLUMN branding_logo_storage_key TEXT;

-- +goose Down

ALTER TABLE profiles
  DROP COLUMN IF EXISTS branding_logo_storage_key;

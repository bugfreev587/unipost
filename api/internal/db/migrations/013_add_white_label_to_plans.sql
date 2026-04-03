-- +goose Up
ALTER TABLE plans ADD COLUMN white_label BOOLEAN NOT NULL DEFAULT TRUE;
UPDATE plans SET white_label = FALSE WHERE id = 'free';

-- +goose Down
ALTER TABLE plans DROP COLUMN IF EXISTS white_label;

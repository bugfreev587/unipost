-- +goose Up
-- Gate X / Twitter publishing behind paid plans. Same shape as the
-- white_label flag (013): bool column on plans, default true so any
-- newly-inserted plan stays permissive, only the free row is flipped.
-- Enforced at validate / publish / connect time via
-- Checker.PlanAllowsPlatform — already-connected X accounts on free
-- workspaces stay connected but cannot publish.
ALTER TABLE plans ADD COLUMN allow_twitter BOOLEAN NOT NULL DEFAULT TRUE;
UPDATE plans SET allow_twitter = FALSE WHERE id = 'free';

-- +goose Down
ALTER TABLE plans DROP COLUMN IF EXISTS allow_twitter;

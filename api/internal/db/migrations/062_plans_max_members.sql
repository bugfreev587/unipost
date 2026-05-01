-- +goose Up
-- RBAC Phase 4 supporting cap (May 2026): max_members per plan.
--
-- The pricing page (post-058) advertises a per-tier team-member cap:
--
--   Free / API / Basic — 1 member (just the owner)
--   Growth             — 3 members
--   Team / Enterprise  — unlimited
--
-- This migration adds the column + populates per the ladder. The
-- members invite handler enforces it at invite time; existing
-- memberships are NEVER retroactively pruned by a downgrade — only
-- new invites are blocked. Same semantics as max_profiles (059).

ALTER TABLE plans ADD COLUMN max_members INTEGER;

UPDATE plans SET max_members = 1 WHERE id = 'free';
UPDATE plans SET max_members = 1 WHERE id = 'api';
UPDATE plans SET max_members = 1 WHERE id = 'basic';
UPDATE plans SET max_members = 3 WHERE id = 'growth';
-- team and enterprise stay NULL (= unlimited)

-- +goose Down
ALTER TABLE plans DROP COLUMN IF EXISTS max_members;

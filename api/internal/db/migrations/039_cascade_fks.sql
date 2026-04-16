-- +goose Up

-- Fix foreign keys that block user/workspace/account deletion.
--
-- Migration 025 added CASCADE for workspace-level FKs but several
-- tables created earlier (or after 025) kept NO ACTION rules. When
-- a user deletes their account, Clerk fires user.deleted → we run
-- DeleteUser which cascades through workspaces/profiles but stops
-- at these intermediate tables, leaving the DB in a partial-delete
-- state while Clerk thinks the user is gone.
--
-- Caught by the bugfreev587@gmail.com deletion:
--   ERROR: update or delete on table "social_posts" violates foreign
--   key constraint "social_post_results_post_id_fkey"

-- social_post_results.post_id → social_posts: CASCADE (child rows)
ALTER TABLE social_post_results
  DROP CONSTRAINT IF EXISTS social_post_results_post_id_fkey,
  ADD CONSTRAINT social_post_results_post_id_fkey
    FOREIGN KEY (post_id) REFERENCES social_posts(id) ON DELETE CASCADE;

-- social_post_results.social_account_id → social_accounts: CASCADE
ALTER TABLE social_post_results
  DROP CONSTRAINT IF EXISTS social_post_results_social_account_id_fkey,
  ADD CONSTRAINT social_post_results_social_account_id_fkey
    FOREIGN KEY (social_account_id) REFERENCES social_accounts(id) ON DELETE CASCADE;

-- inbox_items.social_account_id → social_accounts: CASCADE (items belong to account)
ALTER TABLE inbox_items
  DROP CONSTRAINT IF EXISTS inbox_items_social_account_id_fkey,
  ADD CONSTRAINT inbox_items_social_account_id_fkey
    FOREIGN KEY (social_account_id) REFERENCES social_accounts(id) ON DELETE CASCADE;

-- inbox_items.linked_post_id → social_posts: SET NULL (post link is just a hint)
ALTER TABLE inbox_items
  DROP CONSTRAINT IF EXISTS inbox_items_linked_post_id_fkey,
  ADD CONSTRAINT inbox_items_linked_post_id_fkey
    FOREIGN KEY (linked_post_id) REFERENCES social_posts(id) ON DELETE SET NULL;

-- oauth_states.profile_id → profiles: CASCADE (pending state dies with profile)
-- Migration 009 created this table with the column originally named
-- project_id, so the FK was named oauth_states_project_id_fkey. A later
-- refactor renamed the column but kept the original constraint name,
-- so we drop both possible names defensively.
ALTER TABLE oauth_states
  DROP CONSTRAINT IF EXISTS oauth_states_profile_id_fkey,
  DROP CONSTRAINT IF EXISTS oauth_states_project_id_fkey,
  ADD CONSTRAINT oauth_states_profile_id_fkey
    FOREIGN KEY (profile_id) REFERENCES profiles(id) ON DELETE CASCADE;

-- connect_sessions.completed_social_account_id → social_accounts: SET NULL
-- (the session is a historical audit record; we don't want to delete it
-- just because the account was later removed, but we need to clear the
-- reference so the user/workspace delete can proceed)
ALTER TABLE connect_sessions
  DROP CONSTRAINT IF EXISTS connect_sessions_completed_social_account_id_fkey,
  ADD CONSTRAINT connect_sessions_completed_social_account_id_fkey
    FOREIGN KEY (completed_social_account_id) REFERENCES social_accounts(id) ON DELETE SET NULL;

-- +goose Down

ALTER TABLE social_post_results
  DROP CONSTRAINT IF EXISTS social_post_results_post_id_fkey,
  ADD CONSTRAINT social_post_results_post_id_fkey
    FOREIGN KEY (post_id) REFERENCES social_posts(id);

ALTER TABLE social_post_results
  DROP CONSTRAINT IF EXISTS social_post_results_social_account_id_fkey,
  ADD CONSTRAINT social_post_results_social_account_id_fkey
    FOREIGN KEY (social_account_id) REFERENCES social_accounts(id);

ALTER TABLE inbox_items
  DROP CONSTRAINT IF EXISTS inbox_items_social_account_id_fkey,
  ADD CONSTRAINT inbox_items_social_account_id_fkey
    FOREIGN KEY (social_account_id) REFERENCES social_accounts(id);

ALTER TABLE inbox_items
  DROP CONSTRAINT IF EXISTS inbox_items_linked_post_id_fkey,
  ADD CONSTRAINT inbox_items_linked_post_id_fkey
    FOREIGN KEY (linked_post_id) REFERENCES social_posts(id);

ALTER TABLE oauth_states
  DROP CONSTRAINT IF EXISTS oauth_states_profile_id_fkey,
  ADD CONSTRAINT oauth_states_profile_id_fkey
    FOREIGN KEY (profile_id) REFERENCES profiles(id);

ALTER TABLE connect_sessions
  DROP CONSTRAINT IF EXISTS connect_sessions_completed_social_account_id_fkey,
  ADD CONSTRAINT connect_sessions_completed_social_account_id_fkey
    FOREIGN KEY (completed_social_account_id) REFERENCES social_accounts(id);

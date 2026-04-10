-- +goose Up
--
-- Workspace + Profile architecture refactor.
--
-- Introduces `workspaces` as the top-level security boundary (API keys,
-- billing, posts, media). Renames `projects` to `profiles` — lightweight
-- brand-grouping containers that hold social accounts.
--
-- Tables that move to workspace_id (security/billing scope):
--   api_keys, social_posts, webhooks, platform_credentials,
--   subscriptions, usage, media
--
-- Tables that move to profile_id (brand/account scope):
--   social_accounts, oauth_states, connect_sessions
--
-- Migration is safe to run: 3 registered users, 0 API keys in use.

-- ============================================================
-- 1. Create workspaces table
-- ============================================================
CREATE TABLE workspaces (
  id                        TEXT PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id                   TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name                      TEXT NOT NULL DEFAULT 'Default',
  per_account_monthly_limit INTEGER,
  created_at                TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at                TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_workspaces_user_id ON workspaces(user_id);

-- Seed a default workspace for every existing user.
INSERT INTO workspaces (id, user_id, name)
SELECT gen_random_uuid(), id, 'Default'
FROM users;

-- ============================================================
-- 2. Rename projects → profiles, wire up workspace_id
-- ============================================================
ALTER TABLE projects RENAME TO profiles;

-- Add workspace_id (nullable temporarily for population).
ALTER TABLE profiles
  ADD COLUMN workspace_id TEXT REFERENCES workspaces(id) ON DELETE CASCADE;

-- Populate workspace_id via owner_id → workspaces.user_id.
UPDATE profiles p
SET workspace_id = w.id
FROM workspaces w
WHERE p.owner_id = w.user_id;

-- Now enforce NOT NULL and drop the old owner_id column.
ALTER TABLE profiles ALTER COLUMN workspace_id SET NOT NULL;
ALTER TABLE profiles DROP COLUMN owner_id;

-- Move per_account_monthly_limit to workspace (copy value first).
UPDATE workspaces w
SET per_account_monthly_limit = p.per_account_monthly_limit
FROM profiles p
WHERE p.workspace_id = w.id
  AND p.per_account_monthly_limit IS NOT NULL;

ALTER TABLE profiles DROP COLUMN per_account_monthly_limit;

-- Replace old index.
DROP INDEX IF EXISTS idx_projects_owner_id;
CREATE INDEX idx_profiles_workspace_id ON profiles(workspace_id);

-- ============================================================
-- 3. api_keys: project_id → workspace_id (points to workspaces)
-- ============================================================
-- Add workspace_id column.
ALTER TABLE api_keys
  ADD COLUMN workspace_id TEXT REFERENCES workspaces(id) ON DELETE CASCADE;

-- Populate from profiles lookup.
UPDATE api_keys ak
SET workspace_id = p.workspace_id
FROM profiles p
WHERE ak.project_id = p.id;

-- For any api_keys that couldn't be mapped (orphans), this is a no-op
-- since we have 0 api_keys in production.

ALTER TABLE api_keys ALTER COLUMN workspace_id SET NOT NULL;
ALTER TABLE api_keys DROP COLUMN project_id;

DROP INDEX IF EXISTS idx_api_keys_project_id;
CREATE INDEX idx_api_keys_workspace_id ON api_keys(workspace_id);

-- ============================================================
-- 4. social_accounts: project_id → profile_id (same table, renamed)
-- ============================================================
ALTER TABLE social_accounts RENAME COLUMN project_id TO profile_id;

DROP INDEX IF EXISTS idx_social_accounts_project_id;
CREATE INDEX idx_social_accounts_profile_id ON social_accounts(profile_id);

-- Recreate indexes from migration 020 with profile_id.
DROP INDEX IF EXISTS social_accounts_managed_unique_idx;
CREATE UNIQUE INDEX social_accounts_managed_unique_idx
  ON social_accounts (profile_id, platform, external_user_id)
  WHERE external_user_id IS NOT NULL AND platform <> 'bluesky';

DROP INDEX IF EXISTS social_accounts_ext_user_idx;
CREATE INDEX social_accounts_ext_user_idx
  ON social_accounts (profile_id, external_user_id)
  WHERE external_user_id IS NOT NULL;

-- ============================================================
-- 5. social_posts: project_id → workspace_id (points to workspaces)
-- ============================================================
ALTER TABLE social_posts
  ADD COLUMN workspace_id TEXT REFERENCES workspaces(id) ON DELETE CASCADE;

-- Populate from profiles lookup.
UPDATE social_posts sp
SET workspace_id = p.workspace_id
FROM profiles p
WHERE sp.project_id = p.id;

ALTER TABLE social_posts ALTER COLUMN workspace_id SET NOT NULL;
ALTER TABLE social_posts DROP COLUMN project_id;

-- Recreate indexes from migrations 018 and 019 with workspace_id.
DROP INDEX IF EXISTS social_posts_project_idempotency_uniq;
CREATE UNIQUE INDEX social_posts_workspace_idempotency_uniq
  ON social_posts (workspace_id, idempotency_key)
  WHERE idempotency_key IS NOT NULL;

DROP INDEX IF EXISTS social_posts_project_created_idx;
CREATE INDEX social_posts_workspace_created_idx
  ON social_posts (workspace_id, created_at DESC, id DESC);

-- ============================================================
-- 6. webhooks: project_id → workspace_id
-- ============================================================
ALTER TABLE webhooks
  ADD COLUMN workspace_id TEXT REFERENCES workspaces(id) ON DELETE CASCADE;

UPDATE webhooks wh
SET workspace_id = p.workspace_id
FROM profiles p
WHERE wh.project_id = p.id;

ALTER TABLE webhooks ALTER COLUMN workspace_id SET NOT NULL;
ALTER TABLE webhooks DROP COLUMN project_id;

DROP INDEX IF EXISTS idx_webhooks_project_id;
CREATE INDEX idx_webhooks_workspace_id ON webhooks(workspace_id);

-- ============================================================
-- 7. platform_credentials: project_id → workspace_id
-- ============================================================
-- Drop the old unique constraint first (it references project_id).
ALTER TABLE platform_credentials
  DROP CONSTRAINT IF EXISTS platform_credentials_project_id_platform_key;

ALTER TABLE platform_credentials
  ADD COLUMN workspace_id TEXT REFERENCES workspaces(id) ON DELETE CASCADE;

UPDATE platform_credentials pc
SET workspace_id = p.workspace_id
FROM profiles p
WHERE pc.project_id = p.id;

ALTER TABLE platform_credentials ALTER COLUMN workspace_id SET NOT NULL;
ALTER TABLE platform_credentials DROP COLUMN project_id;

ALTER TABLE platform_credentials
  ADD CONSTRAINT platform_credentials_workspace_id_platform_key UNIQUE(workspace_id, platform);

-- ============================================================
-- 8. subscriptions: project_id → workspace_id
-- ============================================================
-- Drop old unique constraint on project_id (from UNIQUE in CREATE TABLE).
ALTER TABLE subscriptions
  DROP CONSTRAINT IF EXISTS subscriptions_project_id_key;

ALTER TABLE subscriptions
  ADD COLUMN workspace_id TEXT REFERENCES workspaces(id) ON DELETE CASCADE;

UPDATE subscriptions s
SET workspace_id = p.workspace_id
FROM profiles p
WHERE s.project_id = p.id;

ALTER TABLE subscriptions ALTER COLUMN workspace_id SET NOT NULL;
ALTER TABLE subscriptions DROP COLUMN project_id;

ALTER TABLE subscriptions
  ADD CONSTRAINT subscriptions_workspace_id_key UNIQUE(workspace_id);

DROP INDEX IF EXISTS idx_subscriptions_project_id;
CREATE INDEX idx_subscriptions_workspace_id ON subscriptions(workspace_id);

-- ============================================================
-- 9. usage: project_id → workspace_id
-- ============================================================
-- Drop old unique constraint.
ALTER TABLE usage
  DROP CONSTRAINT IF EXISTS usage_project_id_period_key;

ALTER TABLE usage
  ADD COLUMN workspace_id TEXT REFERENCES workspaces(id) ON DELETE CASCADE;

UPDATE usage u
SET workspace_id = p.workspace_id
FROM profiles p
WHERE u.project_id = p.id;

ALTER TABLE usage ALTER COLUMN workspace_id SET NOT NULL;
ALTER TABLE usage DROP COLUMN project_id;

ALTER TABLE usage
  ADD CONSTRAINT usage_workspace_id_period_key UNIQUE(workspace_id, period);

DROP INDEX IF EXISTS idx_usage_project_period;
CREATE INDEX idx_usage_workspace_period ON usage(workspace_id, period);

-- ============================================================
-- 10. media: project_id → workspace_id
-- ============================================================
ALTER TABLE media
  ADD COLUMN workspace_id TEXT REFERENCES workspaces(id) ON DELETE CASCADE;

UPDATE media m
SET workspace_id = p.workspace_id
FROM profiles p
WHERE m.project_id = p.id;

ALTER TABLE media ALTER COLUMN workspace_id SET NOT NULL;
ALTER TABLE media DROP COLUMN project_id;

DROP INDEX IF EXISTS media_project_status_idx;
CREATE INDEX media_workspace_status_idx ON media (workspace_id, status);

-- ============================================================
-- 11. oauth_states: project_id → profile_id
-- ============================================================
ALTER TABLE oauth_states RENAME COLUMN project_id TO profile_id;

-- ============================================================
-- 12. connect_sessions: project_id → profile_id
-- ============================================================
ALTER TABLE connect_sessions RENAME COLUMN project_id TO profile_id;

DROP INDEX IF EXISTS connect_sessions_project_idx;
CREATE INDEX connect_sessions_profile_idx
  ON connect_sessions (profile_id, created_at DESC);

-- ============================================================
-- 13. users: rename project FK columns
-- ============================================================
ALTER TABLE users RENAME COLUMN default_project_id TO default_profile_id;
ALTER TABLE users RENAME COLUMN last_project_id TO last_profile_id;


-- +goose Down
--
-- Reverse the entire migration. This is destructive and should only be
-- used in development.

-- 13. users: restore column names
ALTER TABLE users RENAME COLUMN default_profile_id TO default_project_id;
ALTER TABLE users RENAME COLUMN last_profile_id TO last_project_id;

-- 12. connect_sessions: profile_id → project_id
DROP INDEX IF EXISTS connect_sessions_profile_idx;
CREATE INDEX connect_sessions_project_idx
  ON connect_sessions (project_id, created_at DESC);
ALTER TABLE connect_sessions RENAME COLUMN profile_id TO project_id;

-- 11. oauth_states: profile_id → project_id
ALTER TABLE oauth_states RENAME COLUMN profile_id TO project_id;

-- 10. media: workspace_id → project_id
ALTER TABLE media ADD COLUMN project_id TEXT;
UPDATE media m SET project_id = (
  SELECT p.id FROM profiles p WHERE p.workspace_id = m.workspace_id LIMIT 1
);
ALTER TABLE media ALTER COLUMN project_id SET NOT NULL;
ALTER TABLE media DROP COLUMN workspace_id;
DROP INDEX IF EXISTS media_workspace_status_idx;
CREATE INDEX media_project_status_idx ON media (project_id, status);

-- 9. usage: workspace_id → project_id
ALTER TABLE usage ADD COLUMN project_id TEXT;
UPDATE usage u SET project_id = (
  SELECT p.id FROM profiles p WHERE p.workspace_id = u.workspace_id LIMIT 1
);
ALTER TABLE usage ALTER COLUMN project_id SET NOT NULL;
ALTER TABLE usage DROP COLUMN workspace_id;
ALTER TABLE usage DROP CONSTRAINT IF EXISTS usage_workspace_id_period_key;
ALTER TABLE usage ADD CONSTRAINT usage_project_id_period_key UNIQUE(project_id, period);
DROP INDEX IF EXISTS idx_usage_workspace_period;
CREATE INDEX idx_usage_project_period ON usage(project_id, period);

-- 8. subscriptions: workspace_id → project_id
ALTER TABLE subscriptions ADD COLUMN project_id TEXT;
UPDATE subscriptions s SET project_id = (
  SELECT p.id FROM profiles p WHERE p.workspace_id = s.workspace_id LIMIT 1
);
ALTER TABLE subscriptions ALTER COLUMN project_id SET NOT NULL;
ALTER TABLE subscriptions DROP COLUMN workspace_id;
ALTER TABLE subscriptions DROP CONSTRAINT IF EXISTS subscriptions_workspace_id_key;
ALTER TABLE subscriptions ADD CONSTRAINT subscriptions_project_id_key UNIQUE(project_id);
DROP INDEX IF EXISTS idx_subscriptions_workspace_id;
CREATE INDEX idx_subscriptions_project_id ON subscriptions(project_id);

-- 7. platform_credentials: workspace_id → project_id
ALTER TABLE platform_credentials ADD COLUMN project_id TEXT;
UPDATE platform_credentials pc SET project_id = (
  SELECT p.id FROM profiles p WHERE p.workspace_id = pc.workspace_id LIMIT 1
);
ALTER TABLE platform_credentials ALTER COLUMN project_id SET NOT NULL;
ALTER TABLE platform_credentials DROP COLUMN workspace_id;
ALTER TABLE platform_credentials DROP CONSTRAINT IF EXISTS platform_credentials_workspace_id_platform_key;
ALTER TABLE platform_credentials ADD CONSTRAINT platform_credentials_project_id_platform_key UNIQUE(project_id, platform);

-- 6. webhooks: workspace_id → project_id
ALTER TABLE webhooks ADD COLUMN project_id TEXT;
UPDATE webhooks wh SET project_id = (
  SELECT p.id FROM profiles p WHERE p.workspace_id = wh.workspace_id LIMIT 1
);
ALTER TABLE webhooks ALTER COLUMN project_id SET NOT NULL;
ALTER TABLE webhooks DROP COLUMN workspace_id;
DROP INDEX IF EXISTS idx_webhooks_workspace_id;
CREATE INDEX idx_webhooks_project_id ON webhooks(project_id);

-- 5. social_posts: workspace_id → project_id
ALTER TABLE social_posts ADD COLUMN project_id TEXT;
UPDATE social_posts sp SET project_id = (
  SELECT p.id FROM profiles p WHERE p.workspace_id = sp.workspace_id LIMIT 1
);
ALTER TABLE social_posts ALTER COLUMN project_id SET NOT NULL;
ALTER TABLE social_posts DROP COLUMN workspace_id;
DROP INDEX IF EXISTS social_posts_workspace_idempotency_uniq;
CREATE UNIQUE INDEX social_posts_project_idempotency_uniq
  ON social_posts (project_id, idempotency_key)
  WHERE idempotency_key IS NOT NULL;
DROP INDEX IF EXISTS social_posts_workspace_created_idx;
CREATE INDEX social_posts_project_created_idx
  ON social_posts (project_id, created_at DESC, id DESC);

-- 4. social_accounts: profile_id → project_id
ALTER TABLE social_accounts RENAME COLUMN profile_id TO project_id;
DROP INDEX IF EXISTS idx_social_accounts_profile_id;
CREATE INDEX idx_social_accounts_project_id ON social_accounts(project_id);
DROP INDEX IF EXISTS social_accounts_managed_unique_idx;
CREATE UNIQUE INDEX social_accounts_managed_unique_idx
  ON social_accounts (project_id, platform, external_user_id)
  WHERE external_user_id IS NOT NULL AND platform <> 'bluesky';
DROP INDEX IF EXISTS social_accounts_ext_user_idx;
CREATE INDEX social_accounts_ext_user_idx
  ON social_accounts (project_id, external_user_id)
  WHERE external_user_id IS NOT NULL;

-- 3. api_keys: workspace_id → project_id
ALTER TABLE api_keys ADD COLUMN project_id TEXT;
UPDATE api_keys ak SET project_id = (
  SELECT p.id FROM profiles p WHERE p.workspace_id = ak.workspace_id LIMIT 1
);
ALTER TABLE api_keys DROP COLUMN workspace_id;
DROP INDEX IF EXISTS idx_api_keys_workspace_id;
CREATE INDEX idx_api_keys_project_id ON api_keys(project_id);

-- 2. profiles → projects, restore owner_id
ALTER TABLE profiles ADD COLUMN owner_id TEXT;
UPDATE profiles p SET owner_id = w.user_id
FROM workspaces w WHERE p.workspace_id = w.id;
ALTER TABLE profiles ALTER COLUMN owner_id SET NOT NULL;
ALTER TABLE profiles ADD COLUMN per_account_monthly_limit INTEGER;
UPDATE profiles p SET per_account_monthly_limit = w.per_account_monthly_limit
FROM workspaces w WHERE p.workspace_id = w.id;
ALTER TABLE profiles DROP COLUMN workspace_id;
DROP INDEX IF EXISTS idx_profiles_workspace_id;
CREATE INDEX idx_projects_owner_id ON profiles(owner_id);
ALTER TABLE profiles RENAME TO projects;

-- 1. Drop workspaces
DROP TABLE IF EXISTS workspaces;

-- +goose Up
--
-- "Default project" + "last visited project" tracking on the users
-- table, plus removal of the legacy projects.mode column.
--
-- default_project_id: every signed-up user gets one auto-created
-- "Default" project (lazy: created on the first /me/bootstrap call).
-- The DELETE handler refuses to drop this row, so it's a guaranteed
-- fallback when the dashboard root resolver has nowhere else to go.
--
-- last_project_id: updated as a side-effect of GET /v1/projects/{id},
-- which is the last action the dashboard takes before rendering a
-- project page. We persist on the users row (not on projects) because
-- ownership is 1:1 today — putting it on projects would require a
-- per-project last_visited_at + MAX() lookup on every redirect.
--
-- Both columns reference projects(id) ON DELETE SET NULL so the row
-- self-heals if a project is hard-deleted out from under it (the
-- protection above prevents this for default_project_id, but the
-- FK is still the right safety net).
--
-- projects.mode is dropped: Sprint 1's quickstart-vs-whitelabel split
-- has been superseded by per-account connection_type (BYO/managed),
-- so the project-level flag is dead weight and confuses the projects
-- list UI ("hello world  quickstart" badge no longer means anything).

ALTER TABLE users
  ADD COLUMN default_project_id TEXT REFERENCES projects(id) ON DELETE SET NULL,
  ADD COLUMN last_project_id    TEXT REFERENCES projects(id) ON DELETE SET NULL;

ALTER TABLE projects DROP COLUMN mode;

-- +goose Down
ALTER TABLE projects ADD COLUMN mode TEXT NOT NULL DEFAULT 'quickstart';

ALTER TABLE users
  DROP COLUMN IF EXISTS last_project_id,
  DROP COLUMN IF EXISTS default_project_id;

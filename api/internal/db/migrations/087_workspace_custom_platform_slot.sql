-- +goose Up
--
-- Shared one-platform custom slot for Basic workspaces.
--
-- Basic can customize one platform across both Hosted Connect branding
-- and Platform Credentials. Growth, Team, and Enterprise continue to use all
-- supported platforms. Existing workspaces with exactly one platform
-- credential are backfilled to that platform so their current setup
-- keeps working after the runtime gate starts consulting this column.

ALTER TABLE workspaces
  ADD COLUMN custom_platform_slot TEXT;

UPDATE plans
SET white_label = TRUE
WHERE id = 'basic';

UPDATE workspaces w
SET custom_platform_slot = pc.platform
FROM (
  SELECT workspace_id, MIN(platform) AS platform
  FROM platform_credentials
  GROUP BY workspace_id
  HAVING COUNT(*) = 1
) pc
WHERE pc.workspace_id = w.id;

-- +goose Down

UPDATE plans
SET white_label = FALSE
WHERE id = 'basic';

ALTER TABLE workspaces
  DROP COLUMN IF EXISTS custom_platform_slot;

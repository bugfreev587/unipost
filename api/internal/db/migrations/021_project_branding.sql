-- +goose Up
--
-- Sprint 4 PR4: white-label Connect branding columns on projects.
--
-- Customers who want their end users to see "Connect your Twitter to
-- AcmeCorp" on the hosted Connect page (instead of "Powered by
-- UniPost") set these three optional columns. The values are read by
-- the public connect-session GET endpoint and rendered by the
-- dashboard /connect/[platform] page.
--
-- All three are nullable; the hosted page falls back to UniPost
-- defaults when any of them is null. The "Powered by UniPost" footer
-- is ALWAYS visible regardless of branding — that's the line between
-- white-label (logo + name + color) and full-label (custom domain,
-- footer removal), which is deferred to Sprint 5+ per Sprint 4 D7.
--
-- Validation rules enforced by the API:
--   - logo_url       must be https:// and ≤ 512 chars
--   - display_name   ≤ 60 chars
--   - primary_color  must be a 6-hex-digit color (e.g. "#10b981")
-- The validation lives in the handler; the schema only stores the raw
-- text so we can refine the rules without DB migrations.

ALTER TABLE projects
  ADD COLUMN branding_logo_url      TEXT,
  ADD COLUMN branding_display_name  TEXT,
  ADD COLUMN branding_primary_color TEXT;

-- +goose Down
ALTER TABLE projects
  DROP COLUMN IF EXISTS branding_primary_color,
  DROP COLUMN IF EXISTS branding_display_name,
  DROP COLUMN IF EXISTS branding_logo_url;

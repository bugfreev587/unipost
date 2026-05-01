-- +goose Up
-- Pricing-redesign feature gates (May 2026): make Inbox, Analytics,
-- and profile-count match the marketing copy from migration 058 + the
-- pricing page. Without these columns the live pricing claims "Inbox
-- is paid-only", "Analytics is paid-only", and "Free supports 1
-- profile" are not enforced server-side.
--
-- Gate semantics:
--   allow_inbox       — workspace can call /v1/inbox/* and the WS
--                        notification stream. Free + API tiers cannot.
--   allow_analytics   — workspace can call /v1/analytics/* and the
--                        per-post analytics endpoint. Free cannot;
--                        API gets full read access (the "read-only"
--                        framing is a dashboard-side distinction —
--                        the API endpoints don't mutate anyway).
--   max_profiles      — soft cap enforced at profile-create time.
--                        NULL = unlimited. Existing profiles are
--                        never retroactively pruned by a downgrade;
--                        the gate only blocks NEW creation.

ALTER TABLE plans ADD COLUMN allow_inbox     BOOLEAN NOT NULL DEFAULT TRUE;
ALTER TABLE plans ADD COLUMN allow_analytics BOOLEAN NOT NULL DEFAULT TRUE;
ALTER TABLE plans ADD COLUMN max_profiles    INTEGER;

-- Inbox: Basic and up only.
UPDATE plans SET allow_inbox = FALSE WHERE id IN ('free', 'api');

-- Analytics: API and up.
UPDATE plans SET allow_analytics = FALSE WHERE id = 'free';

-- Profile caps per the pricing ladder. Team and Enterprise stay NULL
-- (= unlimited).
UPDATE plans SET max_profiles = 1  WHERE id = 'free';
UPDATE plans SET max_profiles = 2  WHERE id = 'api';
UPDATE plans SET max_profiles = 5  WHERE id = 'basic';
UPDATE plans SET max_profiles = 25 WHERE id = 'growth';

-- +goose Down
ALTER TABLE plans DROP COLUMN IF EXISTS max_profiles;
ALTER TABLE plans DROP COLUMN IF EXISTS allow_analytics;
ALTER TABLE plans DROP COLUMN IF EXISTS allow_inbox;

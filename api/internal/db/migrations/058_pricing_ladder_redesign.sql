-- +goose Up
-- Pricing redesign (May 2026): switch from per-volume tiers (p10..p1000)
-- to product-tier ladder (Free / API / Basic / Growth / Team / Enterprise).
--
-- Background: see docs/prd-pricing-packaging-redesign.md and the
-- iteration in chat — UniPost product shape (API + dashboard + Inbox +
-- Analytics) was being packaged like PostForMe (per-post-volume only),
-- which underpriced the operational surfaces. New ladder communicates
-- product maturity stages and gates Inbox/Analytics/White-label as
-- step-up unlocks.
--
-- Migration safety: as of 2026-04-30 there are 0 paid customers and
-- the only existing subscription rows reference 'free'. To be defensive,
-- this migration first remaps any subscription on a legacy paid plan
-- to 'free' before deleting the legacy rows. Anyone running this on
-- a fork with paid customers would see those subs degrade to free —
-- intentional, since the plan IDs no longer exist.

-- Step 1: insert the new plans.
-- Columns:
--   price_cents = monthly price in cents (NULL/0 for Free; Enterprise = NULL = "contact us")
--   post_limit  = monthly post quota
--   white_label = whether the plan unlocks White-label / native mode (Sprint 4 PR4 flag)
--   allow_twitter = whether the plan can publish to / connect X (migration 057)
--
-- Note: stripe_price_id stays NULL here — env-var sync (cmd/api/main.go
-- syncStripePriceIDs) populates it at startup once the new STRIPE_PRICE_ID_*
-- env vars are configured in production.
INSERT INTO plans (id, name, price_cents, post_limit, stripe_price_id, white_label, allow_twitter) VALUES
  ('api',        'API',        1000,   1000,  NULL, FALSE, TRUE),
  ('basic',      'Basic',      1900,   2500,  NULL, FALSE, TRUE),
  ('growth',     'Growth',     5900,   7500,  NULL, TRUE,  TRUE),
  ('team',       'Team',       14900,  25000, NULL, TRUE,  TRUE),
  ('enterprise', 'Enterprise', 0,      -1,    NULL, TRUE,  TRUE)
ON CONFLICT (id) DO NOTHING;

-- Step 2: defensively remap any subscription still on a legacy plan
-- to 'free'. The subscriptions.plan_id FK would otherwise block the
-- DELETE in step 3.
UPDATE subscriptions
   SET plan_id = 'free', updated_at = NOW()
 WHERE plan_id IN ('p10','p25','p50','p75','p150','p300','p500','p1000');

-- Step 3: drop legacy plans.
DELETE FROM plans WHERE id IN ('p10','p25','p50','p75','p150','p300','p500','p1000');

-- +goose Down
-- Re-create legacy plans (without stripe_price_id since env-var sync
-- handles those at runtime).
INSERT INTO plans (id, name, price_cents, post_limit, stripe_price_id, white_label, allow_twitter) VALUES
  ('p10',   '$10/mo',    1000,   1000,   NULL, TRUE, TRUE),
  ('p25',   '$25/mo',    2500,   2500,   NULL, TRUE, TRUE),
  ('p50',   '$50/mo',    5000,   5000,   NULL, TRUE, TRUE),
  ('p75',   '$75/mo',    7500,   10000,  NULL, TRUE, TRUE),
  ('p150',  '$150/mo',   15000,  20000,  NULL, TRUE, TRUE),
  ('p300',  '$300/mo',   30000,  40000,  NULL, TRUE, TRUE),
  ('p500',  '$500/mo',   50000,  100000, NULL, TRUE, TRUE),
  ('p1000', '$1000/mo',  100000, 200000, NULL, TRUE, TRUE)
ON CONFLICT (id) DO NOTHING;

DELETE FROM plans WHERE id IN ('api','basic','growth','team','enterprise');

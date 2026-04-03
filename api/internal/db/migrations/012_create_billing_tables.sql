-- +goose Up
CREATE TABLE plans (
  id              TEXT PRIMARY KEY,
  name            TEXT NOT NULL,
  price_cents     INTEGER NOT NULL,
  post_limit      INTEGER NOT NULL,
  stripe_price_id TEXT,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO plans (id, name, price_cents, post_limit, stripe_price_id) VALUES
  ('free',  'Free',      0,      100,    NULL),
  ('p10',   '$10/mo',    1000,   1000,   NULL),
  ('p25',   '$25/mo',    2500,   2500,   NULL),
  ('p50',   '$50/mo',    5000,   5000,   NULL),
  ('p75',   '$75/mo',    7500,   10000,  NULL),
  ('p150',  '$150/mo',   15000,  20000,  NULL),
  ('p300',  '$300/mo',   30000,  40000,  NULL),
  ('p500',  '$500/mo',   50000,  100000, NULL),
  ('p1000', '$1000/mo',  100000, 200000, NULL);

CREATE TABLE subscriptions (
  id                     TEXT PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id             TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE UNIQUE,
  plan_id                TEXT NOT NULL REFERENCES plans(id) DEFAULT 'free',
  stripe_customer_id     TEXT,
  stripe_subscription_id TEXT,
  status                 TEXT NOT NULL DEFAULT 'active',
  current_period_start   TIMESTAMPTZ,
  current_period_end     TIMESTAMPTZ,
  cancel_at_period_end   BOOLEAN DEFAULT FALSE,
  created_at             TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at             TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_subscriptions_project_id ON subscriptions(project_id);
CREATE INDEX idx_subscriptions_stripe_customer_id ON subscriptions(stripe_customer_id);

CREATE TABLE usage (
  id          TEXT PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id  TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  period      TEXT NOT NULL,
  post_count  INTEGER NOT NULL DEFAULT 0,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(project_id, period)
);

CREATE INDEX idx_usage_project_period ON usage(project_id, period);

-- Auto-create free subscription for existing projects
INSERT INTO subscriptions (project_id, plan_id, status)
SELECT id, 'free', 'active' FROM projects
ON CONFLICT (project_id) DO NOTHING;

-- +goose Down
DROP TABLE IF EXISTS usage;
DROP TABLE IF EXISTS subscriptions;
DROP TABLE IF EXISTS plans;

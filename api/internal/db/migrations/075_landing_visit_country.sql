-- +goose Up

ALTER TABLE landing_visits
  ADD COLUMN IF NOT EXISTS country_code TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_landing_visits_country_created
  ON landing_visits (country_code, created_at DESC)
  WHERE country_code <> '';

-- +goose Down

DROP INDEX IF EXISTS idx_landing_visits_country_created;

ALTER TABLE landing_visits
  DROP COLUMN IF EXISTS country_code;

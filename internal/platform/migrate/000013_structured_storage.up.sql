ALTER TABLE runs ADD COLUMN IF NOT EXISTS output TEXT NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS plan_resolutions (
  plan_id UUID NOT NULL REFERENCES plans(id) ON DELETE CASCADE,
  item_key VARCHAR(128) NOT NULL,
  resolution TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (plan_id, item_key)
);

CREATE INDEX IF NOT EXISTS idx_plan_resolutions_plan_id ON plan_resolutions(plan_id);

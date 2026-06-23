CREATE TABLE IF NOT EXISTS project_settings (
  project_id UUID PRIMARY KEY,
  settings JSONB,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

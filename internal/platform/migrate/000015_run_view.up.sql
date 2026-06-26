-- Run view state (replaces run_events)

CREATE TABLE IF NOT EXISTS run_views (
    run_id UUID PRIMARY KEY REFERENCES runs(id) ON DELETE CASCADE,
    seq BIGINT NOT NULL DEFAULT 0,
    state JSONB NOT NULL DEFAULT '{}',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_run_views_updated ON run_views(updated_at);

DROP TABLE IF EXISTS run_events;

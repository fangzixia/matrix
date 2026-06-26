DROP TABLE IF EXISTS run_views;

CREATE TABLE IF NOT EXISTS run_events (
    id UUID PRIMARY KEY,
    run_id UUID NOT NULL,
    step_id UUID,
    event_type VARCHAR(64) NOT NULL,
    payload JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_run_events_run ON run_events(run_id);

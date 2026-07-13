CREATE TABLE IF NOT EXISTS run_view_events (
    job_id    UUID NOT NULL,
    seq       BIGINT NOT NULL,
    event_type TEXT NOT NULL,
    event     JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (job_id, seq)
);

CREATE INDEX IF NOT EXISTS idx_run_view_events_job_seq ON run_view_events (job_id, seq);

CREATE TABLE IF NOT EXISTS run_jobs (
    id UUID PRIMARY KEY,
    run_id UUID NOT NULL UNIQUE,
    status VARCHAR(32) NOT NULL,
    locked_by VARCHAR(128),
    locked_at TIMESTAMPTZ,
    attempts INT DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_run_jobs_status ON run_jobs(status);

CREATE TABLE IF NOT EXISTS run_steps (
    id UUID PRIMARY KEY,
    run_id UUID NOT NULL,
    kind VARCHAR(32) NOT NULL,
    sequence INT NOT NULL,
    status VARCHAR(32) NOT NULL,
    output_summary TEXT,
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_run_steps_run ON run_steps(run_id);

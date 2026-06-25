-- Revert 000008 (best-effort; plans content lives on disk)
ALTER TABLE runs DROP COLUMN IF EXISTS file_path;

ALTER TABLE artifacts DROP COLUMN IF EXISTS title;
ALTER TABLE artifacts DROP COLUMN IF EXISTS plan_path;
ALTER TABLE artifacts DROP COLUMN IF EXISTS run_id;
ALTER TABLE artifacts DROP COLUMN IF EXISTS repository_id;
ALTER TABLE artifacts ADD COLUMN IF NOT EXISTS content TEXT;

CREATE TABLE IF NOT EXISTS requirements (
    id UUID PRIMARY KEY,
    project_id UUID NOT NULL,
    path VARCHAR(512),
    title VARCHAR(512),
    content TEXT,
    updated_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ
);

INSERT INTO requirements (id, project_id, path, title, updated_at, created_at)
SELECT id, project_id, path, title, updated_at, created_at FROM plans;

DROP TABLE IF EXISTS plans;

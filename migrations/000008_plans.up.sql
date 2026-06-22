-- Rename requirements → plans metadata index; slim artifacts; add runs.file_path
-- Version: 000008

CREATE TABLE IF NOT EXISTS plans (
    id UUID PRIMARY KEY,
    project_id UUID NOT NULL,
    repository_id UUID,
    run_id UUID,
    path VARCHAR(512) NOT NULL,
    title VARCHAR(512),
    updated_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_plans_project_id ON plans(project_id);
CREATE INDEX IF NOT EXISTS idx_plans_path ON plans(project_id, path);

-- Migrate legacy requirements rows (drop content column semantics).
INSERT INTO plans (id, project_id, path, title, updated_at, created_at)
SELECT id, project_id, path, title, updated_at, created_at
FROM requirements
WHERE EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'requirements')
  AND NOT EXISTS (SELECT 1 FROM plans p WHERE p.id = requirements.id);

DROP TABLE IF EXISTS requirements;

ALTER TABLE artifacts DROP COLUMN IF EXISTS content;
ALTER TABLE artifacts ADD COLUMN IF NOT EXISTS repository_id UUID;
ALTER TABLE artifacts ADD COLUMN IF NOT EXISTS run_id UUID;
ALTER TABLE artifacts ADD COLUMN IF NOT EXISTS plan_path VARCHAR(512);
ALTER TABLE artifacts ADD COLUMN IF NOT EXISTS title VARCHAR(512);

ALTER TABLE runs ADD COLUMN IF NOT EXISTS file_path VARCHAR(512);

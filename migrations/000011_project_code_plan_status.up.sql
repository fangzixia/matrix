-- Project code uniqueness and plan approval status
-- Version: 000011

CREATE UNIQUE INDEX IF NOT EXISTS idx_projects_path_unique
    ON projects(path)
    WHERE path IS NOT NULL AND path <> '';

ALTER TABLE plans ADD COLUMN IF NOT EXISTS status VARCHAR(32) DEFAULT 'draft';
ALTER TABLE plans ADD COLUMN IF NOT EXISTS resolutions TEXT;

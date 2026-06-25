DROP INDEX IF EXISTS idx_projects_path_unique;
ALTER TABLE plans DROP COLUMN IF EXISTS resolutions;
ALTER TABLE plans DROP COLUMN IF EXISTS status;

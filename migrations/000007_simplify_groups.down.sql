ALTER TABLE groups ADD COLUMN IF NOT EXISTS path VARCHAR(255);
ALTER TABLE groups ADD COLUMN IF NOT EXISTS parent_id UUID;
ALTER TABLE groups ADD COLUMN IF NOT EXISTS visibility VARCHAR(16) DEFAULT 'private';

UPDATE groups SET path = id::text WHERE path IS NULL OR path = '';
ALTER TABLE groups ALTER COLUMN path SET NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS groups_path_key ON groups(path);
CREATE INDEX IF NOT EXISTS idx_groups_parent ON groups(parent_id);

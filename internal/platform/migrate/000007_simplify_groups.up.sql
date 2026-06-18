-- Simplify groups: permission-only, drop GitLab-style path/hierarchy/visibility

DROP INDEX IF EXISTS idx_groups_parent;
ALTER TABLE groups DROP COLUMN IF EXISTS path;
ALTER TABLE groups DROP COLUMN IF EXISTS parent_id;
ALTER TABLE groups DROP COLUMN IF EXISTS visibility;

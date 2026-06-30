-- Drop worktree merge fields from runs
-- Version: 000016

ALTER TABLE runs DROP COLUMN IF EXISTS merge_status;
ALTER TABLE runs DROP COLUMN IF EXISTS run_branch;

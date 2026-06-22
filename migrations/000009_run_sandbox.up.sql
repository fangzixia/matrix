-- Run sandbox worktree fields
-- Version: 000009

ALTER TABLE runs ADD COLUMN IF NOT EXISTS sandbox_path VARCHAR(1024);
ALTER TABLE runs ADD COLUMN IF NOT EXISTS run_branch VARCHAR(128);
ALTER TABLE runs ADD COLUMN IF NOT EXISTS merge_status VARCHAR(32);

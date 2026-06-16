-- baseline indexes (GORM AutoMigrate creates tables)
CREATE INDEX IF NOT EXISTS idx_runs_project_status ON runs(project_id, status);
CREATE INDEX IF NOT EXISTS idx_project_members_user ON project_members(user_id);

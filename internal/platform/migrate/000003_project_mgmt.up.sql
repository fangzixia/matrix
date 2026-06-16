-- Project management enhancement (embed fallback)
CREATE TABLE IF NOT EXISTS groups (
    id UUID PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    path VARCHAR(255) NOT NULL UNIQUE,
    parent_id UUID,
    visibility VARCHAR(16) DEFAULT 'private',
    owner_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_groups_parent ON groups(parent_id);

CREATE TABLE IF NOT EXISTS group_members (
    group_id UUID NOT NULL,
    user_id UUID NOT NULL,
    role VARCHAR(32) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (group_id, user_id)
);

ALTER TABLE projects ADD COLUMN IF NOT EXISTS group_id UUID;
ALTER TABLE projects ADD COLUMN IF NOT EXISTS path VARCHAR(255);

CREATE TABLE IF NOT EXISTS project_repositories (
    id UUID PRIMARY KEY,
    project_id UUID NOT NULL,
    name VARCHAR(128) NOT NULL,
    git_url VARCHAR(512),
    git_branch VARCHAR(128) DEFAULT 'main',
    is_default BOOLEAN DEFAULT FALSE,
    auth_type VARCHAR(32),
    credential_ref VARCHAR(512),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_project_repositories_project ON project_repositories(project_id);

ALTER TABLE runs ADD COLUMN IF NOT EXISTS repository_id UUID;
ALTER TABLE runs ADD COLUMN IF NOT EXISTS error_message TEXT;

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

CREATE TABLE IF NOT EXISTS run_events (
    id UUID PRIMARY KEY,
    run_id UUID NOT NULL,
    step_id UUID,
    event_type VARCHAR(64) NOT NULL,
    payload JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_run_events_run ON run_events(run_id, created_at);

CREATE TABLE IF NOT EXISTS run_jobs (
    id UUID PRIMARY KEY,
    run_id UUID NOT NULL UNIQUE,
    status VARCHAR(32) NOT NULL,
    locked_by VARCHAR(128),
    locked_at TIMESTAMPTZ,
    attempts INT DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_run_jobs_status ON run_jobs(status);

CREATE TABLE IF NOT EXISTS notifications (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL,
    kind VARCHAR(64) NOT NULL,
    title VARCHAR(512),
    body TEXT,
    link VARCHAR(1024),
    read_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_notifications_user ON notifications(user_id, created_at DESC);

-- 系统配置表：按业务域（ai/mcp/git/worker/pipeline）分行存储 JSON 设置
CREATE TABLE IF NOT EXISTS system_settings (  id TEXT PRIMARY KEY,
  settings JSONB NOT NULL DEFAULT '{}',
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

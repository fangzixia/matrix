package models

import "time"

// SystemSetting 是系统级配置域（ai/mcp/git 等）的 JSON 行。
type SystemSetting struct {
	ID        string `gorm:"primaryKey;size:32"`
	Settings  string `gorm:"type:jsonb;not null"`
	UpdatedAt time.Time
}

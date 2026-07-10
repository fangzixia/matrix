package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Run 是一次 AI 任务或 Chat 执行记录。
type Run struct {
	ID           uuid.UUID  `gorm:"type:uuid;primaryKey"`
	ProjectID    uuid.UUID  `gorm:"type:uuid;index;not null"`
	RepositoryID *uuid.UUID `gorm:"type:uuid;index"`
	Kind         string     `gorm:"size:32;not null"`
	Status       string     `gorm:"size:32;not null;index"`
	ModelID      string     `gorm:"size:64"`
	CreatedBy    uuid.UUID  `gorm:"type:uuid;index"`
	AuditPath    string     `gorm:"size:1024"`
	Title        string     `gorm:"size:512"`
	FilePath     string     `gorm:"size:512"`
	SandboxPath  string     `gorm:"size:1024"`
	// SourceSandboxRunID 为 verify/implement 复用的代码沙箱来源 Run（不入库）。
	SourceSandboxRunID uuid.UUID  `gorm:"-" json:"source_sandbox_run_id,omitempty"`
	Output             string     `gorm:"type:text"`
	ChatSessionID      *uuid.UUID `gorm:"type:uuid;index"`
	ChatUserMessageID  *uuid.UUID `gorm:"type:uuid"`
	ErrorMessage       string     `gorm:"type:text"`
	StartedAt          *time.Time
	FinishedAt         *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// BeforeCreate 在 Run 入库前生成 UUID 主键。
func (r *Run) BeforeCreate(tx *gorm.DB) error {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	return nil
}

// RunView 是 Run 活动视图的持久化快照。
type RunView struct {
	RunID     uuid.UUID `gorm:"type:uuid;primaryKey"`
	Seq       int64     `gorm:"not null;default:0"`
	State     string    `gorm:"type:jsonb;not null"`
	UpdatedAt time.Time
}

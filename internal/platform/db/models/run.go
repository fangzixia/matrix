package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Run 是一次 AI 任务或 Chat 执行记录。
type Run struct {
	ID             uuid.UUID  `gorm:"type:uuid;primaryKey"`
	ProjectID      uuid.UUID  `gorm:"type:uuid;index;not null"`
	RepositoryID   *uuid.UUID `gorm:"type:uuid;index"`
	Kind           string     `gorm:"size:32;not null"`
	Status         string     `gorm:"size:32;not null;index"`
	CreatedBy      uuid.UUID  `gorm:"type:uuid;index"`
	AuditPath      string     `gorm:"size:1024"`
	Title          string     `gorm:"size:512"`
	FilePath       string     `gorm:"size:512"`
	EvalFilePath   string     `gorm:"size:512"`
	SandboxPath    string     `gorm:"size:1024"`
	RunBranch      string     `gorm:"size:128"`
	MergeStatus    string     `gorm:"size:32"`
	PipelineStages string     `gorm:"type:jsonb"`
	ErrorMessage   string     `gorm:"type:text"`
	StartedAt      *time.Time
	FinishedAt     *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// BeforeCreate 在 Run 入库前生成 UUID 主键。
func (r *Run) BeforeCreate(tx *gorm.DB) error {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	return nil
}

// RunStep 是 Run 内的流水线或 Harness 步骤。
type RunStep struct {
	ID            uuid.UUID `gorm:"type:uuid;primaryKey"`
	RunID         uuid.UUID `gorm:"type:uuid;index;not null"`
	Kind          string    `gorm:"size:32;not null"`
	Sequence      int       `gorm:"not null"`
	Status        string    `gorm:"size:32;not null"`
	OutputSummary string    `gorm:"type:text"`
	StartedAt     *time.Time
	FinishedAt    *time.Time
	CreatedAt     time.Time
}

// BeforeCreate 在 RunStep 入库前生成 UUID 主键。
func (s *RunStep) BeforeCreate(tx *gorm.DB) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	return nil
}

// RunEvent 是 Run 执行过程中的事件快照。
type RunEvent struct {
	ID        uuid.UUID  `gorm:"type:uuid;primaryKey"`
	RunID     uuid.UUID  `gorm:"type:uuid;index;not null"`
	StepID    *uuid.UUID `gorm:"type:uuid;index"`
	EventType string     `gorm:"size:64;not null"`
	Payload   string     `gorm:"type:jsonb"`
	CreatedAt time.Time
}

// BeforeCreate 在 RunEvent 入库前生成 UUID 主键。
func (e *RunEvent) BeforeCreate(tx *gorm.DB) error {
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	return nil
}

// RunJob 是 Run 对应的异步任务队列条目。
type RunJob struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	RunID     uuid.UUID `gorm:"type:uuid;uniqueIndex;not null"`
	Status    string    `gorm:"size:32;not null;index"`
	LockedBy  string    `gorm:"size:128"`
	LockedAt  *time.Time
	Attempts  int `gorm:"default:0"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

// BeforeCreate 在 RunJob 入库前生成 UUID 主键。
func (j *RunJob) BeforeCreate(tx *gorm.DB) error {
	if j.ID == uuid.Nil {
		j.ID = uuid.New()
	}
	return nil
}

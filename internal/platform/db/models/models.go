// Package models 定义 GORM 持久化实体与 AutoMigrate 注册表。
package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// User 是平台用户账户。
type User struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey"`
	Username     string    `gorm:"uniqueIndex;size:64;not null"`
	Email        string    `gorm:"uniqueIndex;size:255;not null"`
	PasswordHash string    `gorm:"not null"`
	Name         string    `gorm:"size:128"`
	AvatarURL    string    `gorm:"size:512"`
	IsAdmin      bool      `gorm:"default:false"`
	State        string    `gorm:"size:16;default:active"`
	LastSignInAt *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// BeforeCreate 在 User 入库前生成 UUID 主键。
func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	return nil
}

// Session 是用户登录 Session 记录。
type Session struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID    uuid.UUID `gorm:"type:uuid;index;not null"`
	TokenHash string    `gorm:"uniqueIndex;not null"`
	IP        string    `gorm:"size:64"`
	UserAgent string    `gorm:"size:512"`
	ExpiresAt time.Time `gorm:"index"`
	CreatedAt time.Time
}

// BeforeCreate 在 Session 入库前生成 UUID 主键。
func (s *Session) BeforeCreate(tx *gorm.DB) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	return nil
}

// Group 是用户组，用于项目权限继承。
type Group struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	Name      string    `gorm:"size:255;not null"`
	OwnerID   uuid.UUID `gorm:"type:uuid;index"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

// BeforeCreate 在 Group 入库前生成 UUID 主键。
func (g *Group) BeforeCreate(tx *gorm.DB) error {
	if g.ID == uuid.Nil {
		g.ID = uuid.New()
	}
	return nil
}

// GroupMember 是用户组成员关系。
type GroupMember struct {
	GroupID   uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID    uuid.UUID `gorm:"type:uuid;primaryKey"`
	Role      string    `gorm:"size:32;not null"`
	CreatedAt time.Time
}

// Project 是 Git 绑定的业务项目。
type Project struct {
	ID         uuid.UUID  `gorm:"type:uuid;primaryKey"`
	Name       string     `gorm:"size:255;not null"`
	Path       string     `gorm:"size:255"`
	GitURL     string     `gorm:"size:512"`
	GitBranch  string     `gorm:"size:128;default:main"`
	Visibility string     `gorm:"size:16;default:private;index"`
	GroupID    *uuid.UUID `gorm:"type:uuid;index"`
	OwnerID    uuid.UUID  `gorm:"type:uuid;index"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// BeforeCreate 在 Project 入库前生成 UUID 主键。
func (p *Project) BeforeCreate(tx *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return nil
}

// ProjectMember 是项目成员与角色。
type ProjectMember struct {
	ProjectID uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID    uuid.UUID `gorm:"type:uuid;primaryKey"`
	Role      string    `gorm:"size:32;not null"`
	CreatedAt time.Time
}

// ProjectRepository 是项目下的 Git 仓库绑定。
type ProjectRepository struct {
	ID            uuid.UUID `gorm:"type:uuid;primaryKey"`
	ProjectID     uuid.UUID `gorm:"type:uuid;index;not null"`
	Name          string    `gorm:"size:128;not null"`
	GitURL        string    `gorm:"size:512"`
	GitBranch     string    `gorm:"size:128;default:main"`
	IsDefault     bool      `gorm:"default:false"`
	AuthType      string    `gorm:"size:32"`
	CredentialRef string    `gorm:"size:512"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// BeforeCreate 在 ProjectRepository 入库前生成 UUID 主键。
func (r *ProjectRepository) BeforeCreate(tx *gorm.DB) error {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	return nil
}

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

// Notification 是用户站内通知。
type Notification struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID    uuid.UUID `gorm:"type:uuid;index;not null"`
	Kind      string    `gorm:"size:64;not null"`
	Title     string    `gorm:"size:512"`
	Body      string    `gorm:"type:text"`
	Link      string    `gorm:"size:1024"`
	ReadAt    *time.Time
	CreatedAt time.Time
}

// BeforeCreate 在 Notification 入库前生成 UUID 主键。
func (n *Notification) BeforeCreate(tx *gorm.DB) error {
	if n.ID == uuid.Nil {
		n.ID = uuid.New()
	}
	return nil
}

// ChatSession 是项目内 Chat 会话与消息 JSON。
type ChatSession struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	ProjectID uuid.UUID `gorm:"type:uuid;index;not null"`
	Title     string    `gorm:"size:512"`
	Messages  string    `gorm:"type:jsonb"`
	CreatedBy uuid.UUID `gorm:"type:uuid;index"`
	UpdatedAt time.Time
	CreatedAt time.Time
}

// BeforeCreate 在 ChatSession 入库前生成 UUID 主键。
func (c *ChatSession) BeforeCreate(tx *gorm.DB) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	return nil
}

// ProjectSetting 是项目级集成设置 JSON。
type ProjectSetting struct {
	ProjectID uuid.UUID `gorm:"type:uuid;primaryKey"`
	Settings  string    `gorm:"type:jsonb"`
	UpdatedAt time.Time
}

// SystemSetting 是系统级配置域（ai/mcp/git 等）的 JSON 行。
type SystemSetting struct {
	ID        string `gorm:"primaryKey;size:32"`
	Settings  string `gorm:"type:jsonb;not null"`
	UpdatedAt time.Time
}

// Plan 是项目计划文档元数据索引（正文存于工作区 .matrix/PLAN-*.md）。
type Plan struct {
	ID             uuid.UUID  `gorm:"type:uuid;primaryKey"`
	ProjectID      uuid.UUID  `gorm:"type:uuid;index;not null"`
	RepositoryID   *uuid.UUID `gorm:"type:uuid;index"`
	RunID          *uuid.UUID `gorm:"type:uuid;index"`
	Path           string     `gorm:"size:512;not null"`
	Title          string     `gorm:"size:512"`
	UpdatedAt      time.Time
	CreatedAt      time.Time
}

// TableName 指定 GORM 表名。
func (Plan) TableName() string { return "plans" }

// BeforeCreate 在 Plan 入库前生成 UUID 主键。
func (p *Plan) BeforeCreate(tx *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return nil
}

// Artifact 是评测或构建产物元数据索引（正文存于工作区 .matrix/EVAL-PLAN-*.md）。
type Artifact struct {
	ID           uuid.UUID  `gorm:"type:uuid;primaryKey"`
	ProjectID    uuid.UUID  `gorm:"type:uuid;index;not null"`
	RepositoryID *uuid.UUID `gorm:"type:uuid;index"`
	RunID        *uuid.UUID `gorm:"type:uuid;index"`
	Kind         string     `gorm:"size:64;not null"`
	Path         string     `gorm:"size:512;not null"`
	PlanPath     string     `gorm:"size:512"`
	Title        string     `gorm:"size:512"`
	CreatedAt    time.Time
}

// BeforeCreate 在 Artifact 入库前生成 UUID 主键。
func (a *Artifact) BeforeCreate(tx *gorm.DB) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	return nil
}

// All 返回 GORM AutoMigrate 应注册的全部模型指针。
func All() []any {
	return []any{
		&User{}, &Session{}, &Group{}, &GroupMember{},
		&Project{}, &ProjectMember{}, &ProjectRepository{},
		&Run{}, &RunStep{}, &RunEvent{}, &RunJob{},
		&Notification{}, &ChatSession{}, &ProjectSetting{},
		&SystemSetting{},
		&Plan{}, &Artifact{},
	}
}

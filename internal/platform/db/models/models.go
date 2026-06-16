package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

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

func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	return nil
}

type Session struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID    uuid.UUID `gorm:"type:uuid;index;not null"`
	TokenHash string    `gorm:"uniqueIndex;not null"`
	IP        string    `gorm:"size:64"`
	UserAgent string    `gorm:"size:512"`
	ExpiresAt time.Time `gorm:"index"`
	CreatedAt time.Time
}

func (s *Session) BeforeCreate(tx *gorm.DB) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	return nil
}

type Group struct {
	ID         uuid.UUID  `gorm:"type:uuid;primaryKey"`
	Name       string     `gorm:"size:255;not null"`
	Path       string     `gorm:"uniqueIndex;size:255;not null"`
	ParentID   *uuid.UUID `gorm:"type:uuid;index"`
	Visibility string     `gorm:"size:16;default:private"`
	OwnerID    uuid.UUID  `gorm:"type:uuid;index"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func (g *Group) BeforeCreate(tx *gorm.DB) error {
	if g.ID == uuid.Nil {
		g.ID = uuid.New()
	}
	return nil
}

type GroupMember struct {
	GroupID   uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID    uuid.UUID `gorm:"type:uuid;primaryKey"`
	Role      string    `gorm:"size:32;not null"`
	CreatedAt time.Time
}

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

func (p *Project) BeforeCreate(tx *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return nil
}

type ProjectMember struct {
	ProjectID uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID    uuid.UUID `gorm:"type:uuid;primaryKey"`
	Role      string    `gorm:"size:32;not null"`
	CreatedAt time.Time
}

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

func (r *ProjectRepository) BeforeCreate(tx *gorm.DB) error {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	return nil
}

type Run struct {
	ID             uuid.UUID  `gorm:"type:uuid;primaryKey"`
	ProjectID      uuid.UUID  `gorm:"type:uuid;index;not null"`
	RepositoryID   *uuid.UUID `gorm:"type:uuid;index"`
	Kind           string     `gorm:"size:32;not null"`
	Status         string     `gorm:"size:32;not null;index"`
	CreatedBy      uuid.UUID  `gorm:"type:uuid;index"`
	AuditPath      string     `gorm:"size:1024"`
	Title          string     `gorm:"size:512"`
	PipelineStages string     `gorm:"type:jsonb"`
	ErrorMessage   string     `gorm:"type:text"`
	StartedAt      *time.Time
	FinishedAt     *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (r *Run) BeforeCreate(tx *gorm.DB) error {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	return nil
}

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

func (s *RunStep) BeforeCreate(tx *gorm.DB) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	return nil
}

type RunEvent struct {
	ID        uuid.UUID  `gorm:"type:uuid;primaryKey"`
	RunID     uuid.UUID  `gorm:"type:uuid;index;not null"`
	StepID    *uuid.UUID `gorm:"type:uuid;index"`
	EventType string     `gorm:"size:64;not null"`
	Payload   string     `gorm:"type:jsonb"`
	CreatedAt time.Time
}

func (e *RunEvent) BeforeCreate(tx *gorm.DB) error {
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	return nil
}

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

func (j *RunJob) BeforeCreate(tx *gorm.DB) error {
	if j.ID == uuid.Nil {
		j.ID = uuid.New()
	}
	return nil
}

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

func (n *Notification) BeforeCreate(tx *gorm.DB) error {
	if n.ID == uuid.Nil {
		n.ID = uuid.New()
	}
	return nil
}

type ChatSession struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	ProjectID uuid.UUID `gorm:"type:uuid;index;not null"`
	Title     string    `gorm:"size:512"`
	Messages  string    `gorm:"type:jsonb"`
	CreatedBy uuid.UUID `gorm:"type:uuid;index"`
	UpdatedAt time.Time
	CreatedAt time.Time
}

func (c *ChatSession) BeforeCreate(tx *gorm.DB) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	return nil
}

type ProjectSetting struct {
	ProjectID uuid.UUID `gorm:"type:uuid;primaryKey"`
	Settings  string    `gorm:"type:jsonb"`
	UpdatedAt time.Time
}

type SystemSetting struct {
	ID        string `gorm:"primaryKey;size:32"`
	Settings  string `gorm:"type:jsonb;not null"`
	UpdatedAt time.Time
}

type Requirement struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	ProjectID uuid.UUID `gorm:"type:uuid;index"`
	Path      string    `gorm:"size:512"`
	Title     string    `gorm:"size:512"`
	Content   string    `gorm:"type:text"`
	UpdatedAt time.Time
	CreatedAt time.Time
}

func (r *Requirement) BeforeCreate(tx *gorm.DB) error {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	return nil
}

type Artifact struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	ProjectID uuid.UUID `gorm:"type:uuid;index"`
	Kind      string    `gorm:"size:64"`
	Path      string    `gorm:"size:512"`
	Content   string    `gorm:"type:text"`
	CreatedAt time.Time
}

func (a *Artifact) BeforeCreate(tx *gorm.DB) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	return nil
}

func All() []any {
	return []any{
		&User{}, &Session{}, &Group{}, &GroupMember{},
		&Project{}, &ProjectMember{}, &ProjectRepository{},
		&Run{}, &RunStep{}, &RunEvent{}, &RunJob{},
		&Notification{}, &ChatSession{}, &ProjectSetting{},
		&SystemSetting{},
		&Requirement{}, &Artifact{},
	}
}

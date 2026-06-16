// Package run 管理 AI 运行生命周期：入队、执行、步骤/事件持久化与沙箱隔离。
package run

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"matrix/internal/ai/ports"
	"matrix/internal/ai/query"
	"matrix/internal/modules/pipeline"
	"matrix/internal/modules/project"
	"matrix/internal/platform/config"
	"matrix/internal/platform/db/models"
	"matrix/internal/platform/events"
	"matrix/internal/platform/storage"
)

type JobEnqueuer interface {
	Enqueue(ctx context.Context, runID uuid.UUID) error
}

type RunNotifier interface {
	NotifyRunStatus(ctx context.Context, userID uuid.UUID, projectID, runID uuid.UUID, status, title string)
}

type PullAllFunc func(ctx context.Context, projectID uuid.UUID) error

type WorkspaceResolver interface {
	RepoRoot(ctx context.Context, projectID uuid.UUID) (string, error)
	RepoRootFor(ctx context.Context, projectID uuid.UUID, repoID *uuid.UUID) (string, error)
}

type Service struct {
	db        *gorm.DB
	runtime   *Runtime
	hub       *events.Hub
	paths     storage.Paths
	cfg       *config.Config
	workspace WorkspaceResolver
	settings  SettingsResolver
	jobs      JobEnqueuer
	notifier  RunNotifier
	pipeline  *pipeline.Service
	pullAll   PullAllFunc

	sandboxLocks *sandboxLocks
	lifecycleCtx context.Context
	lifecycleMu  sync.RWMutex
}

type SettingsResolver interface {
	GetIntegrations(ctx context.Context, projectID uuid.UUID) (*project.IntegrationSettings, error)
}

type StartInput struct {
	Kind         string
	Title        string
	Message      string
	FilePath     string
	Messages     []query.Message
	RepositoryID *uuid.UUID
	Stages       []string
	Sync         bool
}

func NewService(db *gorm.DB, rt *Runtime, hub *events.Hub, paths storage.Paths, cfg *config.Config, ws WorkspaceResolver, settings SettingsResolver) *Service {
	return &Service{
		db: db, runtime: rt, hub: hub, paths: paths, cfg: cfg,
		workspace: ws, settings: settings,
		sandboxLocks: newSandboxLocks(),
	}
}

func (s *Service) SetJobEnqueuer(j JobEnqueuer)    { s.jobs = j }
func (s *Service) SetNotifier(n RunNotifier)       { s.notifier = n }
func (s *Service) SetPipeline(p *pipeline.Service) { s.pipeline = p }
func (s *Service) SetPullAll(fn PullAllFunc)       { s.pullAll = fn }

func (s *Service) SetLifecycle(ctx context.Context) {
	s.lifecycleMu.Lock()
	s.lifecycleCtx = ctx
	s.lifecycleMu.Unlock()
}

func (s *Service) runCtx() context.Context {
	s.lifecycleMu.RLock()
	ctx := s.lifecycleCtx
	s.lifecycleMu.RUnlock()
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

type RunDTO struct {
	ID           uuid.UUID  `json:"id"`
	ProjectID    uuid.UUID  `json:"project_id"`
	RepositoryID *uuid.UUID `json:"repository_id,omitempty"`
	Kind         string     `json:"kind"`
	Status       string     `json:"status"`
	Title        string     `json:"title,omitempty"`
	AuditPath    string     `json:"audit_path,omitempty"`
	ErrorMessage string     `json:"error_message,omitempty"`
	StartedAt    *time.Time `json:"started_at,omitempty"`
	FinishedAt   *time.Time `json:"finished_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

func (s *Service) List(ctx context.Context, projectID uuid.UUID) ([]RunDTO, error) {
	var rows []models.Run
	if err := s.db.WithContext(ctx).Where("project_id = ?", projectID).Order("created_at desc").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]RunDTO, len(rows))
	for i := range rows {
		out[i] = toRunDTO(&rows[i])
	}
	return out, nil
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (*RunDTO, error) {
	var m models.Run
	if err := s.db.WithContext(ctx).First(&m, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return new(toRunDTO(&m)), nil
}

func (s *Service) Start(ctx context.Context, projectID, userID uuid.UUID, in StartInput) (*RunDTO, error) {
	modelCfg := s.cfg.AI.ActiveModel()
	if s.settings != nil {
		if integ, err := s.settings.GetIntegrations(ctx, projectID); err == nil && integ != nil {
			modelCfg = project.MergeModel(modelCfg, integ.Model)
		}
	}
	if modelCfg.APIKey == "" {
		return nil, errors.New("未配置 API Key")
	}

	kind := in.Kind
	if kind == "" {
		kind = "task"
	}
	if len(in.Stages) > 0 || kind == "pipeline" {
		kind = "pipeline"
	}

	title := in.Title
	if title == "" {
		title = in.Message
	}
	runID := uuid.New()
	status := "queued"
	var startedAt *time.Time
	if in.Sync {
		status = "running"
		startedAt = new(time.Now())
	}

	m := models.Run{
		ID: runID, ProjectID: projectID, RepositoryID: in.RepositoryID,
		Kind: kind, Status: status, CreatedBy: userID, Title: title,
		StartedAt: startedAt,
		AuditPath: storage.RunAuditFile(s.paths, projectID.String(), runID.String()),
	}
	q := s.db.WithContext(ctx)
	if kind == "pipeline" && s.pipeline != nil {
		m.PipelineStages = encodePipelineStages(s.pipeline.ResolveStages(in.Stages))
	} else {
		q = q.Omit("PipelineStages")
	}
	if err := q.Create(&m).Error; err != nil {
		return nil, err
	}

	execCtx := s.runCtx()
	if in.Sync {
		go func() { _ = s.ExecuteRun(execCtx, runID) }()
	} else if s.jobs != nil {
		if err := s.jobs.Enqueue(ctx, runID); err != nil {
			return nil, err
		}
	} else {
		go func() { _ = s.ExecuteRun(execCtx, runID) }()
	}

	return new(toRunDTO(&m)), nil
}

func mcpConfigsToPorts(servers map[string]config.MCPServerYAML) []ports.MCPServerConfig {
	if len(servers) == 0 {
		return nil
	}
	out := make([]ports.MCPServerConfig, 0, len(servers))
	for name, s := range servers {
		if s.Disabled {
			continue
		}
		out = append(out, ports.MCPServerConfig{
			Name: name, Command: s.Command, Args: s.Args, URL: s.URL,
			Headers: s.Headers, Env: s.Env, Disabled: s.Disabled,
		})
	}
	return out
}

func (s *Service) Cancel(ctx context.Context, runID uuid.UUID) error {
	var m models.Run
	if err := s.db.WithContext(ctx).First(&m, "id = ?", runID).Error; err != nil {
		return err
	}
	_ = s.runtime.Cancel(runID.String())
	fin := time.Now()
	return s.db.WithContext(ctx).Model(&m).Updates(map[string]any{"status": "cancelled", "finished_at": fin}).Error
}

func (s *Service) Hub() *events.Hub { return s.hub }

type ChatSessionDTO struct {
	ID        uuid.UUID `json:"id"`
	ProjectID uuid.UUID `json:"project_id"`
	Title     string    `json:"title"`
	Messages  string    `json:"messages"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (s *Service) ListChatSessions(ctx context.Context, projectID uuid.UUID) ([]ChatSessionDTO, error) {
	var rows []models.ChatSession
	if err := s.db.WithContext(ctx).Where("project_id = ?", projectID).Order("updated_at desc").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]ChatSessionDTO, len(rows))
	for i := range rows {
		out[i] = ChatSessionDTO{ID: rows[i].ID, ProjectID: rows[i].ProjectID, Title: rows[i].Title, Messages: rows[i].Messages, UpdatedAt: rows[i].UpdatedAt}
	}
	return out, nil
}

func (s *Service) SaveChatSessions(ctx context.Context, projectID uuid.UUID, userID uuid.UUID, sessions []ChatSessionDTO) error {
	for _, cs := range sessions {
		m := models.ChatSession{
			ID: cs.ID, ProjectID: projectID, Title: cs.Title, Messages: cs.Messages, CreatedBy: userID,
		}
		if m.ID == uuid.Nil {
			m.ID = uuid.New()
		}
		if err := s.db.WithContext(ctx).Save(&m).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) RunChat(ctx context.Context, projectID, userID, sessionID uuid.UUID, userMessage string, history []query.Message) (*RunDTO, error) {
	msgs := append(history, query.Message{Role: "user", Content: userMessage})
	return s.Start(ctx, projectID, userID, StartInput{Kind: "chat", Title: userMessage, Message: userMessage, Messages: msgs})
}

func MessagesFromJSON(raw string) []query.Message {
	if raw == "" {
		return nil
	}
	var msgs []query.Message
	_ = json.Unmarshal([]byte(raw), &msgs)
	return msgs
}

func toRunDTO(m *models.Run) RunDTO {
	return RunDTO{
		ID: m.ID, ProjectID: m.ProjectID, RepositoryID: m.RepositoryID,
		Kind: m.Kind, Status: m.Status, Title: m.Title, AuditPath: m.AuditPath,
		ErrorMessage: m.ErrorMessage, StartedAt: m.StartedAt,
		FinishedAt: m.FinishedAt, CreatedAt: m.CreatedAt,
	}
}

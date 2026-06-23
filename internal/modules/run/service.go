// Package run 管理 AI 运行生命周期：入队、执行、步骤/事件持久化与沙箱隔离。
package run

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"matrix/internal/ai/ports"
	"matrix/internal/ai/query"
	"matrix/internal/modules/artifact"
	"matrix/internal/modules/pipeline"
	"matrix/internal/modules/plan"
	"matrix/internal/modules/workspace"
	"matrix/internal/platform/config"
	"matrix/internal/platform/db/models"
	"matrix/internal/platform/events"
	"matrix/internal/platform/storage"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// JobEnqueuer 将 Run 入队到异步任务队列。
type JobEnqueuer interface {
	Enqueue(ctx context.Context, runID uuid.UUID) error
}

// RunNotifier 在 Run 状态变更时向用户发送通知。
type RunNotifier interface {
	NotifyRunStatus(ctx context.Context, userID uuid.UUID, projectID, runID uuid.UUID, runKind, status, title string)
}

// PullAllFunc 拉取项目下全部 Git 仓库的最新代码。
type PullAllFunc func(ctx context.Context, projectID uuid.UUID) error

// WorkspaceResolver 解析项目或指定仓库的沙箱根目录。
type WorkspaceResolver interface {
	RepoRoot(ctx context.Context, projectID uuid.UUID) (string, error)
	RepoRootFor(ctx context.Context, projectID uuid.UUID, repoID *uuid.UUID) (string, error)
}

// RunSandbox 扩展工作区解析，支持 Run 级 worktree 沙箱、文档目录与合并。
type RunSandbox interface {
	WorkspaceResolver
	ProjectWorkspaceKey(ctx context.Context, projectID uuid.UUID) (string, error)
	CreateRunWorktree(ctx context.Context, projectID uuid.UUID, repositoryID *uuid.UUID, runID uuid.UUID) (sandboxPath, branch string, err error)
	RemoveRunWorktree(ctx context.Context, projectID uuid.UUID, repositoryID *uuid.UUID, runID uuid.UUID, branch, sandboxPath string) error
	MergeRunWorktree(ctx context.Context, projectID uuid.UUID, repositoryID *uuid.UUID, runID uuid.UUID, branch, sandboxPath string) ([]string, error)
	DocsRoot(ctx context.Context, projectID uuid.UUID) (string, error)
	ResolveDocPath(projectID uuid.UUID, logicalPath string) (string, error)
	SanitizeDocLogicalPath(logicalPath string) (string, error)
	DocSandboxDir(ctx context.Context, projectID uuid.UUID) (string, error)
}

// Service 管理 AI 运行生命周期：入队、执行、步骤/事件持久化与沙箱隔离。
type Service struct {
	db           *gorm.DB
	runtime      *Runtime
	hub          *events.Hub
	paths        storage.Paths
	runtimeCfg   *config.RuntimeConfig
	workspace    RunSandbox
	jobs         JobEnqueuer
	notifier     RunNotifier
	pipeline     *pipeline.Service
	pullAll      PullAllFunc
	plans        *plan.Service
	artifacts    *artifact.Service
	sandboxLocks *sandboxLocks
	lifecycleCtx context.Context
	lifecycleMu  sync.RWMutex
}

// NewService 创建 Run 服务实例。
func NewService(db *gorm.DB, rt *Runtime, hub *events.Hub, paths storage.Paths, runtime *config.RuntimeConfig, ws RunSandbox) *Service {
	return &Service{
		db: db, runtime: rt, hub: hub, paths: paths, runtimeCfg: runtime,
		workspace:    ws,
		sandboxLocks: newSandboxLocks(),
	}
}

// StartInput 是启动一次 Run 或 Chat 的请求参数。
type StartInput struct {
	Kind         string
	Title        string
	Message      string
	FilePath     string
	EvalFilePath string
	Messages     []query.Message
	RepositoryID *uuid.UUID
	Stages       []string
	Sync         bool
}

// SetJobEnqueuer 注入异步任务入队器。
func (s *Service) SetJobEnqueuer(j JobEnqueuer) { s.jobs = j }

// SetNotifier 注入 Run 状态通知器。
func (s *Service) SetNotifier(n RunNotifier) { s.notifier = n }

// SetPlans 注入计划文档索引服务。
func (s *Service) SetPlans(p *plan.Service) { s.plans = p }

// SetArtifacts 注入评测产物索引服务。
func (s *Service) SetArtifacts(a *artifact.Service) { s.artifacts = a }

// SetPipeline 注入流水线服务。
func (s *Service) SetPipeline(p *pipeline.Service) { s.pipeline = p }

// SetPullAll 注入批量 Git 拉取函数。
func (s *Service) SetPullAll(fn PullAllFunc) { s.pullAll = fn }

// SetLifecycle 注册进程退出时的 Run 生命周期钩子。
func (s *Service) SetLifecycle(ctx context.Context) {
	s.lifecycleMu.Lock()
	s.lifecycleCtx = ctx
	s.lifecycleMu.Unlock()
}

// runCtx 返回 Run 生命周期上下文。
func (s *Service) runCtx() context.Context {
	s.lifecycleMu.RLock()
	ctx := s.lifecycleCtx
	s.lifecycleMu.RUnlock()
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

// RunDTO 是 Run API 返回的数据传输对象。
type RunDTO struct {
	ID           uuid.UUID  `json:"id"`
	ProjectID    uuid.UUID  `json:"project_id"`
	RepositoryID *uuid.UUID `json:"repository_id,omitempty"`
	Kind         string     `json:"kind"`
	Status       string     `json:"status"`
	Title        string     `json:"title,omitempty"`
	FilePath     string     `json:"file_path,omitempty"`
	EvalFilePath string     `json:"eval_file_path,omitempty"`
	AuditPath    string     `json:"audit_path,omitempty"`
	SandboxPath  string     `json:"sandbox_path,omitempty"`
	RunBranch    string     `json:"run_branch,omitempty"`
	MergeStatus  string     `json:"merge_status,omitempty"`
	ErrorMessage string     `json:"error_message,omitempty"`
	StartedAt    *time.Time `json:"started_at,omitempty"`
	FinishedAt   *time.Time `json:"finished_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

// List 返回列表；kind 为四阶段之一时严格过滤，chat/task/pipeline 等不会混入阶段列表。
func (s *Service) List(ctx context.Context, projectID uuid.UUID, kind string) ([]RunDTO, error) {
	q := s.db.WithContext(ctx).Where("project_id = ?", projectID)
	if kind != "" {
		if !IsStageKind(kind) {
			return []RunDTO{}, nil
		}
		q = q.Where("kind = ?", kind)
	}
	var rows []models.Run
	if err := q.Order("created_at desc").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]RunDTO, len(rows))
	for i := range rows {
		out[i] = toRunDTO(&rows[i])
	}
	return out, nil
}

// Get 执行对应操作。
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*RunDTO, error) {
	var m models.Run
	if err := s.db.WithContext(ctx).First(&m, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return new(toRunDTO(&m)), nil
}

// Start 启动 Run。
func (s *Service) Start(ctx context.Context, projectID, userID uuid.UUID, in StartInput) (*RunDTO, error) {
	modelCfg, ok := s.runtimeCfg.AI.ActiveModel()
	if !ok || !config.ModelConfigured(modelCfg) {
		return nil, errors.New("未配置模型：请在管理区域 → 系统配置中设置并启用默认模型")
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
	filePath, err := s.workspace.SanitizeDocLogicalPath(strings.TrimSpace(in.FilePath))
	if err != nil {
		return nil, fmt.Errorf("file_path: %w", err)
	}
	evalFilePath, err := s.workspace.SanitizeDocLogicalPath(strings.TrimSpace(in.EvalFilePath))
	if err != nil {
		return nil, fmt.Errorf("eval_file_path: %w", err)
	}
	if evalFilePath != "" && kind != "build" {
		return nil, errors.New("eval_file_path 仅用于 build 阶段")
	}
	if filePath != "" && !strings.HasPrefix(filePath, workspace.DocsPlansRel+"/") {
		return nil, errors.New("file_path 必须在 docs/plans/ 下")
	}
	if evalFilePath != "" && !strings.HasPrefix(evalFilePath, workspace.DocsEvaluationsRel+"/") {
		return nil, errors.New("eval_file_path 必须在 docs/evaluations/ 下")
	}
	if RequiresPlanFile(kind) && filePath == "" {
		return nil, errors.New("请选择计划文件")
	}
	if RequiresApprovedPlan(kind) && filePath != "" && s.plans != nil {
		ok, err := s.plans.IsApproved(ctx, projectID, filePath)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, errors.New("计划尚未批准，请先确认风险、冲突与待澄清项")
		}
	}
	projectCode, err := s.workspace.ProjectWorkspaceKey(ctx, projectID)
	if err != nil {
		return nil, err
	}
	m := models.Run{
		ID: runID, ProjectID: projectID, RepositoryID: in.RepositoryID,
		Kind: kind, Status: status, CreatedBy: userID, Title: title,
		FilePath: filePath, EvalFilePath: evalFilePath,
		StartedAt: startedAt,
		AuditPath: storage.RunAuditFile(s.paths, projectCode, runID.String()),
	}
	if kind == "chat" && len(in.Messages) > 0 {
		if b, err := json.Marshal(in.Messages); err == nil {
			m.InputMessages = string(b)
		}
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

// mcpConfigsToPorts 将 MCP YAML 配置转换为运行时端口配置。
func mcpConfigsToPorts(servers map[string]config.MCPServerConfig) []ports.MCPServerConfig {
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

// Cancel 取消进行中的 Run。
func (s *Service) Cancel(ctx context.Context, runID uuid.UUID) error {
	var m models.Run
	if err := s.db.WithContext(ctx).First(&m, "id = ?", runID).Error; err != nil {
		return err
	}
	_ = s.runtime.Cancel(runID.String())
	fin := time.Now()
	return s.db.WithContext(ctx).Model(&m).Updates(map[string]any{"status": "cancelled", "finished_at": fin}).Error
}

// Hub 返回 Run SSE 事件 Hub。
func (s *Service) Hub() *events.Hub { return s.hub }

// ChatSessionDTO 是聊天会话 API 返回的数据传输对象。
type ChatSessionDTO struct {
	ID        uuid.UUID `json:"id"`
	ProjectID uuid.UUID `json:"project_id"`
	Title     string    `json:"title"`
	Messages  string    `json:"messages"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ListChatSessions 返回项目 Chat 会话列表。
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

// SaveChatSessions 持久化 Chat 会话消息。
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

// RunChat 启动或续接 Chat Run。
func (s *Service) RunChat(ctx context.Context, projectID, userID, sessionID uuid.UUID, userMessage string, attachments []query.MessageAttachment, history []query.Message) (*RunDTO, error) {
	userMsg := query.Message{Role: query.RoleUser, Content: userMessage, Attachments: attachments}
	msgs := append(history, userMsg)
	return s.Start(ctx, projectID, userID, StartInput{Kind: "chat", Title: userMessage, Message: userMessage, Messages: msgs})
}

// MessagesFromJSON 将 JSON 字符串反序列化为聊天消息列表。
func MessagesFromJSON(raw string) []query.Message {
	if raw == "" {
		return nil
	}
	var msgs []query.Message
	_ = json.Unmarshal([]byte(raw), &msgs)
	return msgs
}

// toRunDTO 将 Run 模型转换为 API DTO。
func toRunDTO(m *models.Run) RunDTO {
	return RunDTO{
		ID: m.ID, ProjectID: m.ProjectID, RepositoryID: m.RepositoryID,
		Kind: m.Kind, Status: m.Status, Title: m.Title,
		FilePath: m.FilePath, EvalFilePath: m.EvalFilePath,
		AuditPath:   m.AuditPath,
		SandboxPath: m.SandboxPath, RunBranch: m.RunBranch, MergeStatus: m.MergeStatus,
		ErrorMessage: m.ErrorMessage, StartedAt: m.StartedAt,
		FinishedAt: m.FinishedAt, CreatedAt: m.CreatedAt,
	}
}

// MergeRun 将 Run worktree 合并到主仓库。
func (s *Service) MergeRun(ctx context.Context, runID uuid.UUID) (*RunDTO, []string, error) {
	var m models.Run
	if err := s.db.WithContext(ctx).First(&m, "id = ?", runID).Error; err != nil {
		return nil, nil, err
	}
	if m.MergeStatus != "pending" {
		return nil, nil, errors.New("当前 Run 不可合并")
	}
	conflicts, err := s.workspace.MergeRunWorktree(ctx, m.ProjectID, m.RepositoryID, runID, m.RunBranch, m.SandboxPath)
	if err != nil {
		return new(toRunDTO(&m)), conflicts, err
	}
	_ = s.workspace.RemoveRunWorktree(ctx, m.ProjectID, m.RepositoryID, runID, m.RunBranch, m.SandboxPath)
	_ = s.db.WithContext(ctx).Model(&m).Updates(map[string]any{
		"merge_status": "merged", "sandbox_path": "", "run_branch": "",
	}).Error
	m.MergeStatus = "merged"
	m.SandboxPath = ""
	m.RunBranch = ""
	return new(toRunDTO(&m)), nil, nil
}

// DiscardRun 放弃 Run worktree 变更。
func (s *Service) DiscardRun(ctx context.Context, runID uuid.UUID) (*RunDTO, error) {
	var m models.Run
	if err := s.db.WithContext(ctx).First(&m, "id = ?", runID).Error; err != nil {
		return nil, err
	}
	if m.MergeStatus != "pending" {
		return nil, errors.New("当前 Run 不可放弃")
	}
	if m.SandboxPath != "" {
		_ = s.workspace.RemoveRunWorktree(ctx, m.ProjectID, m.RepositoryID, runID, m.RunBranch, m.SandboxPath)
	}
	_ = s.db.WithContext(ctx).Model(&m).Updates(map[string]any{
		"merge_status": "discarded", "sandbox_path": "", "run_branch": "",
	}).Error
	m.MergeStatus = "discarded"
	m.SandboxPath = ""
	m.RunBranch = ""
	return new(toRunDTO(&m)), nil
}

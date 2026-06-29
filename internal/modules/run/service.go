// Package run 管理 AI 运行生命周期：入队、执行、步骤/事件持久化与沙箱隔离。
package run

import (
	"context"
	"errors"
	"fmt"
	"matrix/internal/ai/ports"
	"matrix/internal/ai/query"
	"matrix/internal/modules/artifact"
	"matrix/internal/modules/pipeline"
	"matrix/internal/modules/plan"
	"matrix/internal/modules/run/view"
	"matrix/internal/modules/workspace"
	"matrix/internal/platform/config"
	"matrix/internal/platform/db/models"
	"matrix/internal/platform/events"
	"matrix/internal/platform/logging"
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

// AIRuntimeReloader 在 Run 启动前从数据库刷新 AI 模型配置（含 API Key）。
type AIRuntimeReloader interface {
	ReloadAI(ctx context.Context) error
}

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
	MatrixDir(ctx context.Context, projectID, runID uuid.UUID) (string, error)
}

// Service 管理 AI 运行生命周期：入队、执行、视图投影与沙箱隔离。
type Service struct {
	db           *gorm.DB
	runtime      *Runtime
	hub          *events.Hub
	viewStore    *view.Store
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
	aiReloader   AIRuntimeReloader
}

// NewService 创建 Run 服务实例。
func NewService(db *gorm.DB, rt *Runtime, hub *events.Hub, paths storage.Paths, runtime *config.RuntimeConfig, ws RunSandbox) *Service {
	return &Service{
		db: db, runtime: rt, hub: hub,
		viewStore: view.NewStore(db),
		paths:     paths, runtimeCfg: runtime,
		workspace:    ws,
		sandboxLocks: newSandboxLocks(),
	}
}

// StartInput 是启动一次 Run 或 Chat 的请求参数。
type StartInput struct {
	Kind              string
	Title             string
	Message           string
	FilePath          string
	EvalFilePath      string
	ModelID           string
	Messages          []query.Message
	RepositoryID      *uuid.UUID
	Stages            []string
	Sync              bool
	ChatSessionID     *uuid.UUID
	ChatUserMessageID *uuid.UUID
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

// SetAIRuntimeReloader 注入 AI 配置热加载器，Run 启动前从 DB 刷新模型 Key。
func (s *Service) SetAIRuntimeReloader(r AIRuntimeReloader) { s.aiReloader = r }

func (s *Service) refreshAIRuntime(ctx context.Context) error {
	if s.aiReloader == nil {
		return nil
	}
	return s.aiReloader.ReloadAI(ctx)
}

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
	ID                 uuid.UUID  `json:"id"`
	ProjectID          uuid.UUID  `json:"project_id"`
	RepositoryID       *uuid.UUID `json:"repository_id,omitempty"`
	Kind               string     `json:"kind"`
	Status             string     `json:"status"`
	Title              string     `json:"title,omitempty"`
	FilePath           string     `json:"file_path,omitempty"`
	EvalFilePath       string     `json:"eval_file_path,omitempty"`
	AuditPath          string     `json:"audit_path,omitempty"`
	SandboxPath        string     `json:"sandbox_path,omitempty"`
	RunBranch          string     `json:"run_branch,omitempty"`
	MergeStatus        string     `json:"merge_status,omitempty"`
	ErrorMessage       string     `json:"error_message,omitempty"`
	Output             string     `json:"output,omitempty"`
	StartedAt          *time.Time `json:"started_at,omitempty"`
	FinishedAt         *time.Time `json:"finished_at,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	UserMessageID      string     `json:"user_message_id,omitempty"`
	AssistantMessageID string     `json:"assistant_message_id,omitempty"`
}

// List 返回列表；kind 为四阶段之一时严格过滤，chat/task/pipeline 等不会混入阶段列表。
func (s *Service) List(ctx context.Context, projectID uuid.UUID, kind string) ([]RunDTO, error) {
	q := s.db.WithContext(ctx).Where("project_id = ?", projectID)
	if kind != "" {
		if !shouldNotifyRun(kind) {
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
	// 刷新配置
	if err := s.refreshAIRuntime(ctx); err != nil {
		return nil, fmt.Errorf("刷新 AI 配置失败: %w", err)
	}
	modelID := strings.TrimSpace(in.ModelID)
	// 检查模型有效性
	if _, _, err := s.runtimeCfg.AI.ResolveModel(modelID); err != nil {
		return nil, err
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
		Kind: kind, Status: status, ModelID: modelID, CreatedBy: userID, Title: title,
		FilePath: filePath, EvalFilePath: evalFilePath,
		StartedAt:     startedAt,
		AuditPath:     storage.RunAuditFile(s.paths, projectCode, runID.String()),
		ChatSessionID: in.ChatSessionID, ChatUserMessageID: in.ChatUserMessageID,
	}
	q := s.db.WithContext(ctx)
	if err := q.Create(&m).Error; err != nil {
		return nil, err
	}
	if kind == "pipeline" && s.pipeline != nil {
		stages := s.pipeline.ResolveStages(in.Stages)
		for i, stageKind := range stages {
			step := models.RunStep{
				RunID: runID, Kind: stageKind, Sequence: i + 1, Status: "pending",
			}
			if err := s.db.WithContext(ctx).Create(&step).Error; err != nil {
				return nil, err
			}
		}
	}
	execCtx := s.runCtx()
	if in.Sync {
		logging.Agent("run: 同步派发", "run_id", runID, "kind", kind)
		go func() { _ = s.ExecuteRun(execCtx, runID) }()
	} else if s.jobs != nil {
		if err := s.jobs.Enqueue(ctx, runID); err != nil {
			return nil, err
		}
		logging.Agent("run: 已入队等待执行",
			"run_id", runID, "kind", kind,
			"chat_session_id", in.ChatSessionID,
			"chat_user_message_id", in.ChatUserMessageID,
		)
	} else {
		logging.Agent("run: 进程内直接执行", "run_id", runID, "kind", kind)
		go func() { _ = s.ExecuteRun(execCtx, runID) }()
	}
	return new(toRunDTO(&m)), nil
}

// mcpConfigsToPorts 将 MCP YAML 配置转换为运行时端口配置。
func mcpConfigsToPorts(servers map[string]config.MCPServerConfig, allowCommandMCP bool) []ports.MCPServerConfig {
	if len(servers) == 0 {
		return nil
	}
	out := make([]ports.MCPServerConfig, 0, len(servers))
	for name, s := range servers {
		if s.Disabled {
			continue
		}
		if !allowCommandMCP && s.Command != "" {
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
	if err := s.db.WithContext(ctx).Model(&m).Updates(map[string]any{"status": "cancelled", "finished_at": fin}).Error; err != nil {
		return err
	}
	return s.finishRunView(ctx, runID, "cancelled", "", "任务已取消", "")
}

// Hub 返回通知 SSE 事件 Hub（非 Run 视图流）。
func (s *Service) Hub() *events.Hub { return s.hub }

// StreamCatchUpSince 从 DB 读取 seq > afterSeq 的视图事件，供 SSE 轮询。
func (s *Service) StreamCatchUpSince(
	ctx context.Context,
	runID uuid.UUID,
	mode view.Mode,
	afterSeq int64,
) ([]view.Envelope, bool, int64, error) {
	var m models.Run
	if err := s.db.WithContext(ctx).First(&m, "id = ?", runID).Error; err != nil {
		return nil, false, afterSeq, err
	}
	userErr := ""
	if m.ErrorMessage != "" {
		userErr = view.FormatUserRunError(m.ErrorMessage)
	}
	envs, done, maxSeq := s.viewStore.CatchUpAfterSeq(
		ctx, runID.String(), mode, afterSeq,
		m.Status, m.Output, userErr, m.MergeStatus,
	)
	return envs, done, maxSeq, nil
}

// GetRunView 返回 Run 活动视图快照。
func (s *Service) GetRunView(ctx context.Context, runID uuid.UUID) (*view.RunViewState, error) {
	var m models.Run
	if err := s.db.WithContext(ctx).First(&m, "id = ?", runID).Error; err != nil {
		return nil, err
	}
	return s.viewStore.Snapshot(ctx, runID.String())
}

// ChatMessageDTO 是单条聊天消息 API 返回。
type ChatMessageDTO struct {
	ID          string                    `json:"id"`
	ParentID    *string                   `json:"parent_id"`
	Role        string                    `json:"role"`
	Content     string                    `json:"content"`
	Attachments []query.MessageAttachment `json:"attachments,omitempty"`
	RunID       string                    `json:"run_id,omitempty"`
	CreatedAt   string                    `json:"created_at,omitempty"`
}

// ChatSessionSummaryDTO 是会话列表项（不含消息树）。
type ChatSessionSummaryDTO struct {
	ID           uuid.UUID `json:"id"`
	ProjectID    uuid.UUID `json:"project_id"`
	Title        string    `json:"title"`
	ModelID      string    `json:"model_id,omitempty"`
	ActiveLeafID string    `json:"active_leaf_id,omitempty"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// ChatSessionDTO 是聊天会话详情（含消息树）。
type ChatSessionDTO struct {
	ID           uuid.UUID        `json:"id"`
	ProjectID    uuid.UUID        `json:"project_id"`
	Title        string           `json:"title"`
	ModelID      string           `json:"model_id,omitempty"`
	ActiveLeafID string           `json:"active_leaf_id,omitempty"`
	Nodes        []ChatMessageDTO `json:"nodes"`
	UpdatedAt    time.Time        `json:"updated_at"`
}

func chatSessionSummaryFromRow(row *models.ChatSession) ChatSessionSummaryDTO {
	activeLeaf := ""
	if row.ActiveLeafID != nil {
		activeLeaf = row.ActiveLeafID.String()
	}
	return ChatSessionSummaryDTO{
		ID: row.ID, ProjectID: row.ProjectID, Title: row.Title,
		ModelID: row.ModelID, ActiveLeafID: activeLeaf, UpdatedAt: row.UpdatedAt,
	}
}

func chatSessionDetailFromRow(ctx context.Context, s *Service, row *models.ChatSession) (ChatSessionDTO, error) {
	sm, err := s.LoadSessionTree(ctx, row)
	if err != nil {
		return ChatSessionDTO{}, err
	}
	nodes := make([]ChatMessageDTO, 0, len(sm.Nodes))
	for _, n := range sm.Nodes {
		nodes = append(nodes, ChatMessageDTO{
			ID: n.ID, ParentID: n.ParentID, Role: n.Role, Content: n.Content,
			Attachments: n.Attachments, RunID: n.RunID, CreatedAt: n.CreatedAt,
		})
	}
	return ChatSessionDTO{
		ID: row.ID, ProjectID: row.ProjectID, Title: row.Title,
		ModelID: row.ModelID, ActiveLeafID: sm.ActiveLeafID, Nodes: nodes,
		UpdatedAt: row.UpdatedAt,
	}, nil
}

// CreateChatSession 创建空会话。
func (s *Service) CreateChatSession(ctx context.Context, projectID, userID uuid.UUID, id uuid.UUID, title, modelID string) (*ChatSessionSummaryDTO, error) {
	if id == uuid.Nil {
		id = uuid.New()
	}
	if strings.TrimSpace(title) == "" {
		title = "新对话"
	}
	m := models.ChatSession{
		ID: id, ProjectID: projectID, Title: title,
		ModelID: modelID, CreatedBy: userID,
	}
	if err := s.db.WithContext(ctx).Create(&m).Error; err != nil {
		return nil, err
	}
	dto := chatSessionSummaryFromRow(&m)
	return &dto, nil
}

// UpdateChatSession 更新会话元数据。
func (s *Service) UpdateChatSession(ctx context.Context, projectID, sessionID uuid.UUID, title, modelID string) (*ChatSessionSummaryDTO, error) {
	var row models.ChatSession
	if err := s.db.WithContext(ctx).Where("id = ? AND project_id = ?", sessionID, projectID).First(&row).Error; err != nil {
		return nil, errors.New("会话不存在")
	}
	updates := map[string]any{"updated_at": time.Now()}
	if strings.TrimSpace(title) != "" {
		updates["title"] = strings.TrimSpace(title)
	}
	if strings.TrimSpace(modelID) != "" {
		updates["model_id"] = strings.TrimSpace(modelID)
	}
	if err := s.db.WithContext(ctx).Model(&row).Updates(updates).Error; err != nil {
		return nil, err
	}
	if err := s.db.WithContext(ctx).First(&row, "id = ?", sessionID).Error; err != nil {
		return nil, err
	}
	dto := chatSessionSummaryFromRow(&row)
	return &dto, nil
}

// ListChatSessions 返回项目 Chat 会话列表（仅元数据）。
func (s *Service) ListChatSessions(ctx context.Context, projectID uuid.UUID) ([]ChatSessionSummaryDTO, error) {
	var rows []models.ChatSession
	if err := s.db.WithContext(ctx).Where("project_id = ?", projectID).Order("updated_at desc").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]ChatSessionSummaryDTO, len(rows))
	for i := range rows {
		out[i] = chatSessionSummaryFromRow(&rows[i])
	}
	return out, nil
}

// GetChatSession 返回单个 Chat 会话（含消息树）。
func (s *Service) GetChatSession(ctx context.Context, projectID, sessionID uuid.UUID) (*ChatSessionDTO, error) {
	var row models.ChatSession
	if err := s.db.WithContext(ctx).Where("id = ? AND project_id = ?", sessionID, projectID).First(&row).Error; err != nil {
		return nil, err
	}
	dto, err := chatSessionDetailFromRow(ctx, s, &row)
	if err != nil {
		return nil, err
	}
	return &dto, nil
}

// DeleteChatSession 删除指定 Chat 会话及其消息。
func (s *Service) DeleteChatSession(ctx context.Context, projectID, sessionID uuid.UUID) error {
	var row models.ChatSession
	if err := s.db.WithContext(ctx).Where("id = ? AND project_id = ?", sessionID, projectID).First(&row).Error; err != nil {
		return errors.New("会话不存在")
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("session_id = ?", sessionID).Delete(&models.ChatMessage{}).Error; err != nil {
			return err
		}
		res := tx.Delete(&row)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return errors.New("会话不存在")
		}
		return nil
	})
}

// RunChat 启动或续接 Chat Run。
func (s *Service) RunChat(ctx context.Context, projectID, userID, sessionID, userMessageID uuid.UUID, userMessage string, attachments []query.MessageAttachment, history []query.Message, modelID string) (*RunDTO, error) {
	//组装请求消息
	userMsg := query.Message{Role: query.RoleUser, Content: userMessage, Attachments: attachments}
	//组装上下文
	msgs := append(history, userMsg)
	sid := sessionID
	uid := userMessageID
	return s.Start(ctx, projectID, userID, StartInput{
		Kind: "chat", Title: userMessage, Message: userMessage,
		ModelID: modelID, Messages: msgs,
		ChatSessionID: &sid, ChatUserMessageID: &uid,
	})
}

// toRunDTO 将 Run 模型转换为 API DTO。
func toRunDTO(m *models.Run) RunDTO {
	return RunDTO{
		ID: m.ID, ProjectID: m.ProjectID, RepositoryID: m.RepositoryID,
		Kind: m.Kind, Status: m.Status, Title: m.Title,
		FilePath: m.FilePath, EvalFilePath: m.EvalFilePath,
		AuditPath:   m.AuditPath,
		SandboxPath: m.SandboxPath, RunBranch: m.RunBranch, MergeStatus: m.MergeStatus,
		ErrorMessage: m.ErrorMessage, Output: m.Output, StartedAt: m.StartedAt,
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

// Package run 管理 AI 运行生命周期：启动、执行、视图投影与沙箱隔离。
package run

import (
	"context"
	"errors"
	"fmt"
	"matrix/internal/ai/query"
	"matrix/internal/modules/artifact"
	"matrix/internal/modules/notification"
	"matrix/internal/modules/plan"
	"matrix/internal/modules/run/view"
	"matrix/internal/modules/settings"
	"matrix/internal/modules/workspace"
	"matrix/internal/platform/db/models"
	"matrix/internal/platform/db/repo"
	"matrix/internal/platform/logging"
	"matrix/internal/platform/storage"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Service 管理 AI 运行生命周期：启动、执行、视图投影与沙箱隔离。
type Service struct {
	stores       *repo.Stores
	viewStore    *view.Store
	paths        storage.Paths
	settings     *settings.Service
	workspace    *workspace.ProjectRepoResolver
	notifier     *notification.Service
	plans        *plan.Service
	artifacts    *artifact.Service
	lifecycleCtx context.Context
	lifecycleMu  sync.RWMutex
	runCancelMu  sync.Mutex
	runCancels   map[string]context.CancelFunc
}

// NewService 创建 Run 服务实例。
func NewService(stores *repo.Stores, paths storage.Paths, sysSettings *settings.Service, ws *workspace.ProjectRepoResolver) *Service {
	return &Service{
		stores:    stores,
		viewStore: view.NewStore(stores.Run),
		paths:     paths, settings: sysSettings,
		workspace:  ws,
		runCancels: make(map[string]context.CancelFunc),
	}
}

// StartRunRequest 是 POST /runs 的请求体。
type StartRunRequest struct {
	Kind     Kind   `json:"kind"`
	Message  string `json:"message"`
	FilePath string `json:"file_path"`
}

// StartInput 是启动一次 Run 或 Chat 的请求参数。
type StartInput struct {
	Kind              Kind
	Title             string
	Message           string
	FilePath          string
	ModelID           string
	RepositoryID      *uuid.UUID
	ChatSessionID     *uuid.UUID
	ChatUserMessageID *uuid.UUID
}

// SetNotifier 注入 Run 状态通知器。
func (s *Service) SetNotifier(n *notification.Service) { s.notifier = n }

// SetPlans 注入计划文档索引服务。
func (s *Service) SetPlans(p *plan.Service) { s.plans = p }

// SetArtifacts 注入评测产物索引服务。
func (s *Service) SetArtifacts(a *artifact.Service) { s.artifacts = a }

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
	Kind               Kind       `json:"kind"`
	Status             string     `json:"status"`
	Title              string     `json:"title,omitempty"`
	FilePath           string     `json:"file_path,omitempty"`
	AuditPath          string     `json:"audit_path,omitempty"`
	SandboxPath        string     `json:"sandbox_path,omitempty"`
	ErrorMessage       string     `json:"error_message,omitempty"`
	Output             string     `json:"output,omitempty"`
	StartedAt          *time.Time `json:"started_at,omitempty"`
	FinishedAt         *time.Time `json:"finished_at,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	UserMessageID      string     `json:"user_message_id,omitempty"`
	AssistantMessageID string     `json:"assistant_message_id,omitempty"`
}

// List 返回列表；kind 为流水线阶段之一时严格过滤，chat 等不会混入阶段列表。
func (s *Service) List(ctx context.Context, projectID uuid.UUID, kind string) ([]RunDTO, error) {
	q := s.stores.Run
	rows, err := func() ([]models.Run, error) {
		if kind != "" {
			if !Kind(kind).IsHarness() {
				return []models.Run{}, nil
			}
		}
		return q.ListByProject(ctx, projectID, kind)
	}()
	if err != nil {
		return nil, err
	}
	out := make([]RunDTO, len(rows))
	for i := range rows {
		out[i] = toRunDTO(&rows[i])
	}
	return out, nil
}

// GetForProject 返回项目内指定 Run，避免只按 run_id 访问跨项目资源。
func (s *Service) GetForProject(ctx context.Context, projectID, id uuid.UUID) (*RunDTO, error) {
	m, err := s.loadProjectRun(ctx, projectID, id)
	if err != nil {
		return nil, err
	}
	return new(toRunDTO(m)), nil
}

func (s *Service) loadProjectRun(ctx context.Context, projectID, runID uuid.UUID) (*models.Run, error) {
	return s.stores.Run.GetByProject(ctx, projectID, runID)
}

// Start 启动 Run。
func (s *Service) Start(ctx context.Context, projectID, userID uuid.UUID, in StartInput) (*RunDTO, error) {
	kind := in.Kind
	title := in.Title
	if title == "" {
		title = in.Message
	}
	runID := uuid.New()
	status := "running"
	startedAt := new(time.Now())
	filePath, err := s.workspace.SanitizeDocLogicalPath(strings.TrimSpace(in.FilePath))
	if err != nil {
		return nil, fmt.Errorf("file_path: %w", err)
	}
	if filePath != "" && !strings.HasPrefix(filePath, workspace.DocsPlansRel+"/") {
		return nil, errors.New("file_path 必须在 docs/plans/ 下")
	}
	if RequiresPlanFile(&kind) && filePath == "" {
		return nil, errors.New("请选择计划文件")
	}
	if RequiresApprovedPlan(&kind) && filePath != "" && s.plans != nil {
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
	m, err := s.stores.Run.Start(ctx, repo.StartParams{
		ID: runID, ProjectID: projectID, RepositoryID: in.RepositoryID,
		Kind: string(kind), Status: status, ModelID: in.ModelID, CreatedBy: userID, Title: title,
		FilePath: filePath, AuditPath: storage.RunAuditFile(s.paths, projectCode, runID.String()),
		StartedAt: startedAt, ChatSessionID: in.ChatSessionID, ChatUserMessageID: in.ChatUserMessageID,
	})
	if err != nil {
		return nil, err
	}
	logging.Agent("run: 开始执行", "run_id", runID, "kind", kind)
	go func() { _ = s.ExecuteRun(s.runCtx(), runID) }()
	return new(toRunDTO(m)), nil
}

// CancelForProject 取消项目内指定 Run。
func (s *Service) CancelForProject(ctx context.Context, projectID, runID uuid.UUID) error {
	m, err := s.loadProjectRun(ctx, projectID, runID)
	if err != nil {
		return err
	}
	return s.cancelRunModel(ctx, m)
}

func (s *Service) cancelRunModel(ctx context.Context, m *models.Run) error {
	s.cancelAgentRun(m.ID.String())
	fin := time.Now()
	if err := s.stores.Run.Cancel(ctx, m.ID, fin); err != nil {
		return err
	}
	return s.finishRunView(ctx, m.ID, "cancelled", "", "任务已取消")
}

// StreamCatchUpSinceForProject 从项目内 Run 读取视图事件。
func (s *Service) StreamCatchUpSinceForProject(
	ctx context.Context,
	projectID uuid.UUID,
	runID uuid.UUID,
	mode view.Mode,
	afterSeq int64,
) ([]view.Envelope, bool, int64, error) {
	m, err := s.loadProjectRun(ctx, projectID, runID)
	if err != nil {
		return nil, false, afterSeq, err
	}
	return s.catchUpRunView(ctx, m, mode, afterSeq)
}

func (s *Service) catchUpRunView(ctx context.Context, m *models.Run, mode view.Mode, afterSeq int64) ([]view.Envelope, bool, int64, error) {
	userErr := ""
	if m.ErrorMessage != "" {
		userErr = view.FormatUserRunError(m.ErrorMessage)
	}
	envs, done, maxSeq := s.viewStore.CatchUpAfterSeq(
		ctx, m.ID.String(), mode, afterSeq,
		m.Status, m.Output, userErr,
	)
	return envs, done, maxSeq, nil
}

// GetRunViewForProject 返回项目内 Run 活动视图快照。
func (s *Service) GetRunViewForProject(ctx context.Context, projectID, runID uuid.UUID) (*view.RunViewState, error) {
	if _, err := s.loadProjectRun(ctx, projectID, runID); err != nil {
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
	row, err := s.stores.Chat.CreateSession(ctx, repo.CreateSessionParams{
		ID: id, ProjectID: projectID, Title: title, ModelID: modelID, CreatedBy: userID,
	})
	if err != nil {
		return nil, err
	}
	return new(chatSessionSummaryFromRow(row)), nil
}

// UpdateChatSession 更新会话元数据。
func (s *Service) UpdateChatSession(ctx context.Context, projectID, sessionID uuid.UUID, title, modelID string) (*ChatSessionSummaryDTO, error) {
	if _, err := s.stores.Chat.GetSessionByProject(ctx, projectID, sessionID); err != nil {
		return nil, errors.New("会话不存在")
	}
	titleTrim := strings.TrimSpace(title)
	modelTrim := strings.TrimSpace(modelID)
	row, err := s.stores.Chat.UpdateSession(ctx, sessionID, titleTrim, modelTrim)
	if err != nil {
		return nil, err
	}
	return new(chatSessionSummaryFromRow(row)), nil
}

// ListChatSessions 返回项目 Chat 会话列表（仅元数据）。
func (s *Service) ListChatSessions(ctx context.Context, projectID uuid.UUID) ([]ChatSessionSummaryDTO, error) {
	rows, err := s.stores.Chat.ListSessionsByProject(ctx, projectID)
	if err != nil {
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
	row, err := s.stores.Chat.GetSessionByProject(ctx, projectID, sessionID)
	if err != nil {
		return nil, err
	}
	dto, err := chatSessionDetailFromRow(ctx, s, row)
	if err != nil {
		return nil, err
	}
	return &dto, nil
}

// DeleteChatSession 删除指定 Chat 会话及其消息。
func (s *Service) DeleteChatSession(ctx context.Context, projectID, sessionID uuid.UUID) error {
	return s.stores.Chat.DeleteSession(ctx, projectID, sessionID)
}

// toRunDTO 将 Run 模型转换为 API DTO。
func toRunDTO(m *models.Run) RunDTO {
	errMsg := m.ErrorMessage
	if errMsg != "" {
		errMsg = view.FormatUserRunError(errMsg)
	}
	return RunDTO{
		ID: m.ID, ProjectID: m.ProjectID, RepositoryID: m.RepositoryID,
		Kind: Kind(m.Kind), Status: m.Status, Title: m.Title,
		FilePath:     m.FilePath,
		AuditPath:    m.AuditPath,
		SandboxPath:  m.SandboxPath,
		ErrorMessage: errMsg, Output: m.Output, StartedAt: m.StartedAt,
		FinishedAt: m.FinishedAt, CreatedAt: m.CreatedAt,
	}
}

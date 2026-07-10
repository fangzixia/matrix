package repo

import (
	"context"
	"time"

	"matrix/internal/platform/db/models"

	"github.com/google/uuid"
)

// RunStore 封装 Run 生命周期相关的全部持久化操作。
type RunStore struct {
	c *catalog
}

func newRunStore(c *catalog) *RunStore { return &RunStore{c: c} }

// StartParams 是创建并启动 Run 的持久化参数。
type StartParams struct {
	ID                uuid.UUID
	ProjectID         uuid.UUID
	RepositoryID      *uuid.UUID
	Kind              string
	Status            string
	ModelID           string
	CreatedBy         uuid.UUID
	Title             string
	FilePath          string
	AuditPath         string
	StartedAt         *time.Time
	ChatSessionID     *uuid.UUID
	ChatUserMessageID *uuid.UUID
}

// ListByProject 按项目（及可选 kind）列出 Run。
func (s *RunStore) ListByProject(ctx context.Context, projectID uuid.UUID, kind string) ([]models.Run, error) {
	return s.c.run.ListByProject(ctx, projectID, kind)
}

// GetByProject 按项目与 Run ID 查询。
func (s *RunStore) GetByProject(ctx context.Context, projectID, runID uuid.UUID) (*models.Run, error) {
	return s.c.run.GetByProject(ctx, projectID, runID)
}

// GetByID 按 Run ID 查询。
func (s *RunStore) GetByID(ctx context.Context, runID uuid.UUID) (*models.Run, error) {
	return s.c.run.GetByID(ctx, runID)
}

// Start 创建 Run 记录。
func (s *RunStore) Start(ctx context.Context, p StartParams) (*models.Run, error) {
	m := models.Run{
		ID: p.ID, ProjectID: p.ProjectID, RepositoryID: p.RepositoryID,
		Kind: p.Kind, Status: p.Status, ModelID: p.ModelID, CreatedBy: p.CreatedBy,
		Title: p.Title, FilePath: p.FilePath, AuditPath: p.AuditPath, StartedAt: p.StartedAt,
		ChatSessionID: p.ChatSessionID, ChatUserMessageID: p.ChatUserMessageID,
	}
	if err := s.c.run.Create(ctx, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// Cancel 取消 Run。
func (s *RunStore) Cancel(ctx context.Context, runID uuid.UUID, fin time.Time) error {
	return s.c.run.Cancel(ctx, runID, fin)
}

// MarkRunning 将 Run 标记为运行中。
func (s *RunStore) MarkRunning(ctx context.Context, runID uuid.UUID, now time.Time) error {
	return s.c.run.MarkRunning(ctx, runID, now)
}

// Finalize 写入终态并返回最新 Run（含 output）。
func (s *RunStore) Finalize(ctx context.Context, runID uuid.UUID, status, errMsg string, fin time.Time) (*models.Run, error) {
	if err := s.c.run.Finalize(ctx, runID, status, errMsg, fin); err != nil {
		return nil, err
	}
	return s.c.run.GetByID(ctx, runID)
}

// UpdateSandboxPath 更新沙箱路径。
func (s *RunStore) UpdateSandboxPath(ctx context.Context, runID uuid.UUID, path string) error {
	return s.c.run.UpdateFields(ctx, runID, map[string]any{"sandbox_path": path})
}

// UpdateOutput 更新 Run 输出。
func (s *RunStore) UpdateOutput(ctx context.Context, runID uuid.UUID, output string) error {
	return s.c.run.UpdateOutput(ctx, runID, output)
}

// FindLatestSandboxForPlan 查找同 plan 下最近成功的沙箱来源 Run。
func (s *RunStore) FindLatestSandboxForPlan(
	ctx context.Context,
	projectID uuid.UUID,
	repositoryID *uuid.UUID,
	planPath string,
	excludeRunID uuid.UUID,
	kinds []string,
) (*models.Run, error) {
	return s.c.run.FindLatestSandboxForPlan(ctx, projectID, repositoryID, planPath, excludeRunID, kinds)
}

// ListSteps 列出 Run 步骤。
func (s *RunStore) ListSteps(ctx context.Context, runID uuid.UUID) ([]models.RunStep, error) {
	return s.c.runStep.ListByRunID(ctx, runID)
}

// SaveView 持久化 Run 视图快照。
func (s *RunStore) SaveView(ctx context.Context, row *models.RunView) error {
	return s.c.runView.Save(ctx, row)
}

// LoadView 加载 Run 视图快照；不存在时返回 (nil, nil)。
func (s *RunStore) LoadView(ctx context.Context, runID uuid.UUID) (*models.RunView, error) {
	row, err := s.c.runView.GetByRunID(ctx, runID)
	if IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return row, nil
}

// GetOutput 读取 Run 输出（助手消息等场景）。
func (s *RunStore) GetOutput(ctx context.Context, runID uuid.UUID) (string, error) {
	m, err := s.c.run.GetByID(ctx, runID)
	if err != nil {
		return "", err
	}
	return m.Output, nil
}

// HasAssistantMessage 检查 Run 是否已有助手消息。
func (s *RunStore) HasAssistantMessage(ctx context.Context, runID uuid.UUID) (bool, error) {
	n, err := s.c.chatMessage.CountByRunID(ctx, runID)
	return n > 0, err
}

// SandboxSourceNotFound 判断沙箱来源查询是否为「未找到」类错误。
func (s *RunStore) SandboxSourceNotFound(err error) bool {
	return IsNotFound(err)
}

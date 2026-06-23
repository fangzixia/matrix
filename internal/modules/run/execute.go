package run

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"matrix/internal/ai/harness"
	"matrix/internal/ai/ports"
	"matrix/internal/ai/stream"
	"matrix/internal/modules/eval"
	"matrix/internal/platform/config"
	"matrix/internal/platform/db/models"
	"matrix/internal/platform/storage"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
)

// StepDTO 是流水线步骤 API 返回的数据传输对象。
type StepDTO struct {
	ID            uuid.UUID  `json:"id"`
	RunID         uuid.UUID  `json:"run_id"`
	Kind          string     `json:"kind"`
	Sequence      int        `json:"sequence"`
	Status        string     `json:"status"`
	OutputSummary string     `json:"output_summary,omitempty"`
	StartedAt     *time.Time `json:"started_at,omitempty"`
	FinishedAt    *time.Time `json:"finished_at,omitempty"`
}

// EventDTO 是 Run 事件 API 返回的数据传输对象。
type EventDTO struct {
	ID        uuid.UUID  `json:"id"`
	RunID     uuid.UUID  `json:"run_id"`
	StepID    *uuid.UUID `json:"step_id,omitempty"`
	EventType string     `json:"event_type"`
	Payload   string     `json:"payload,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

// ListSteps 返回 Run 步骤列表。
func (s *Service) ListSteps(ctx context.Context, runID uuid.UUID) ([]StepDTO, error) {
	var rows []models.RunStep
	if err := s.db.WithContext(ctx).Where("run_id = ?", runID).Order("sequence asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]StepDTO, len(rows))
	for i := range rows {
		out[i] = StepDTO{
			ID: rows[i].ID, RunID: rows[i].RunID, Kind: rows[i].Kind, Sequence: rows[i].Sequence,
			Status: rows[i].Status, OutputSummary: rows[i].OutputSummary,
			StartedAt: rows[i].StartedAt, FinishedAt: rows[i].FinishedAt,
		}
	}
	return out, nil
}

// ListEvents 返回 Run 事件列表。
func (s *Service) ListEvents(ctx context.Context, runID uuid.UUID, afterID *uuid.UUID, limit int) ([]EventDTO, error) {
	if limit <= 0 {
		limit = 200
	}
	q := s.db.WithContext(ctx).Where("run_id = ?", runID)
	if afterID != nil && *afterID != uuid.Nil {
		q = q.Where("id > ?", *afterID)
	}
	var rows []models.RunEvent
	if err := q.Order("created_at asc").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]EventDTO, len(rows))
	for i := range rows {
		out[i] = EventDTO{
			ID: rows[i].ID, RunID: rows[i].RunID, StepID: rows[i].StepID,
			EventType: rows[i].EventType, Payload: rows[i].Payload, CreatedAt: rows[i].CreatedAt,
		}
	}
	return out, nil
}

// GetAudit 返回 Run 审计日志路径或内容。
func (s *Service) GetAudit(ctx context.Context, runID uuid.UUID) (string, error) {
	var m models.Run
	if err := s.db.WithContext(ctx).First(&m, "id = ?", runID).Error; err != nil {
		return "", err
	}
	if m.AuditPath == "" {
		return "", nil
	}
	b, err := os.ReadFile(m.AuditPath)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// persistEvent 将 Run 事件持久化到数据库。
func (s *Service) persistEvent(ctx context.Context, runID uuid.UUID, stepID *uuid.UUID, msg stream.Message) {
	payload, _ := json.Marshal(msg)
	ev := models.RunEvent{
		RunID: runID, StepID: stepID, EventType: msg.Type, Payload: string(payload),
	}
	_ = s.db.WithContext(ctx).Create(&ev).Error
}

// compositeSink 组合 SSE Hub 与审计 Sink。
func (s *Service) compositeSink(runID uuid.UUID, stepID *uuid.UUID) stream.Sink {
	base := s.hub.Sink(runID.String())
	return stream.FuncSink(func(ctx context.Context, msg stream.Message) error {
		s.persistEvent(ctx, runID, stepID, msg)
		return base.Publish(ctx, msg)
	})
}

// ExecuteRun 执行 Run 的 Harness 流水线。
func (s *Service) ExecuteRun(ctx context.Context, runID uuid.UUID) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	var m models.Run
	if err := s.db.WithContext(ctx).First(&m, "id = ?", runID).Error; err != nil {
		return err
	}
	switch m.Status {
	case "succeeded", "cancelled":
		return nil
	}
	now := time.Now()
	_ = s.db.WithContext(ctx).Model(&m).Updates(map[string]any{
		"status": "running", "started_at": now, "finished_at": nil, "error_message": "",
	}).Error
	if s.notifier != nil && shouldNotifyRun(m.Kind) {
		s.notifier.NotifyRunStatus(ctx, m.CreatedBy, m.ProjectID, runID, m.Kind, "running", m.Title)
	}
	var runErr error
	switch m.Kind {
	case "build":
		runErr = s.executeBuildLoop(ctx, &m)
	case "pipeline":
		runErr = s.executePipeline(ctx, &m)
	default:
		runErr = s.executeSingle(ctx, &m, m.Kind, nil)
	}
	if ctx.Err() != nil {
		runErr = ctx.Err()
	}
	fin := time.Now()
	status := "succeeded"
	errMsg := ""
	if runErr != nil {
		status = "failed"
		errMsg = runErr.Error()
	}
	updates := map[string]any{
		"status": status, "finished_at": fin, "error_message": errMsg,
	}
	if ms := s.mergeStatusAfterRun(&m, status); ms != "" {
		updates["merge_status"] = ms
	}
	_ = s.db.WithContext(ctx).Model(&models.Run{}).Where("id = ?", runID).Updates(updates).Error
	if runErr != nil && s.runtimeCfg.Run.CleanupOnFailure && m.SandboxPath != "" {
		_ = s.workspace.RemoveRunWorktree(ctx, m.ProjectID, m.RepositoryID, runID, m.RunBranch, m.SandboxPath)
		_ = s.db.WithContext(ctx).Model(&models.Run{}).Where("id = ?", runID).Updates(map[string]any{
			"sandbox_path": "", "run_branch": "",
		}).Error
	}
	if s.notifier != nil && shouldNotifyRun(m.Kind) {
		s.notifier.NotifyRunStatus(ctx, m.CreatedBy, m.ProjectID, runID, m.Kind, status, m.Title)
	}
	s.hub.Publish(runID.String(), stream.ResultSuccessMsg(runID.String(), "", "", 0, time.Since(now)))
	return runErr
}

// prepareRunSandbox 为 Run 准备 worktree 沙箱环境。
func (s *Service) prepareRunSandbox(ctx context.Context, m *models.Run, kind string) error {
	if !UsesWorktreeKind(kind) {
		return nil
	}
	if !s.useWorktreeSandbox() {
		return nil
	}
	if m.SandboxPath != "" {
		return nil
	}
	sandboxPath, branch, err := s.workspace.CreateRunWorktree(ctx, m.ProjectID, m.RepositoryID, m.ID)
	if err != nil {
		return err
	}
	m.SandboxPath = sandboxPath
	m.RunBranch = branch
	return s.db.WithContext(ctx).Model(m).Updates(map[string]any{
		"sandbox_path": sandboxPath, "run_branch": branch,
	}).Error
}

const buildMaxRounds = 5

// executeBuildLoop 执行 build 阶段的闭环迭代。
func (s *Service) executeBuildLoop(ctx context.Context, m *models.Run) error {
	if err := s.prepareRunSandbox(ctx, m, "build"); err != nil {
		return err
	}
	var unlock func()
	if s.shouldUseSharedLock(m) {
		projectCode, err := s.workspace.ProjectWorkspaceKey(ctx, m.ProjectID)
		if err != nil {
			return err
		}
		unlock = s.sandboxLocks.acquire(projectCode, m.RepositoryID)
		defer unlock()
	}
	for round := 1; round <= buildMaxRounds; round++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := s.runHarnessStage(ctx, m, "implement", nil, false); err != nil {
			return fmt.Errorf("构建第 %d 轮编码: %w", round, err)
		}
		if err := s.runHarnessStage(ctx, m, "verify", nil, false); err != nil {
			return fmt.Errorf("构建第 %d 轮验证: %w", round, err)
		}
		docsRoot, err := s.workspace.DocsRoot(ctx, m.ProjectID)
		if err != nil {
			return err
		}
		score, evalPath, ok, err := eval.LatestScore(docsRoot, m.FilePath)
		if err != nil {
			return err
		}
		if ok && score >= eval.PassScore {
			return nil
		}
		if round == buildMaxRounds {
			if !ok {
				return fmt.Errorf("构建未达标：无法解析评测综合分")
			}
			return fmt.Errorf("构建未达标：综合分 %.1f < %.1f（%s）", score, eval.PassScore, evalPath)
		}
	}
	return nil
}

// sandboxDir 解析 Run 的沙箱目录路径。
func (s *Service) sandboxDir(ctx context.Context, m *models.Run) (string, error) {
	if m.SandboxPath != "" {
		return m.SandboxPath, nil
	}
	return s.workspace.RepoRootFor(ctx, m.ProjectID, m.RepositoryID)
}

// loadExistingSteps 加载 Run 已有流水线步骤。
func (s *Service) loadExistingSteps(ctx context.Context, runID uuid.UUID) (map[int]*models.RunStep, error) {
	var rows []models.RunStep
	if err := s.db.WithContext(ctx).Where("run_id = ?", runID).Order("sequence asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	bySeq := make(map[int]*models.RunStep, len(rows))
	for i := range rows {
		bySeq[rows[i].Sequence] = &rows[i]
	}
	return bySeq, nil
}

// executePipeline 按序执行流水线各阶段。
func (s *Service) executePipeline(ctx context.Context, m *models.Run) error {
	if s.pipeline == nil {
		return errors.New("pipeline not configured")
	}
	stages := s.pipeline.StagesForRun(ctx, m.ID, decodePipelineStages(m.PipelineStages))
	existing, err := s.loadExistingSteps(ctx, m.ID)
	if err != nil {
		return err
	}
	pullBefore := s.pipeline.PullBeforeStage() && !s.useWorktreeSandbox()
	if pullBefore && s.pullAll != nil {
		_ = s.pullAll(ctx, m.ProjectID)
	}
	for i, kind := range stages {
		if err := ctx.Err(); err != nil {
			return err
		}
		seq := i + 1
		if prev, ok := existing[seq]; ok && prev.Status == "succeeded" {
			continue
		}
		var stepID uuid.UUID
		start := time.Now()
		if prev, ok := existing[seq]; ok {
			stepID = prev.ID
			_ = s.db.WithContext(ctx).Model(prev).Updates(map[string]any{
				"status": "running", "started_at": start, "finished_at": nil, "output_summary": "",
			}).Error
		} else {
			step := models.RunStep{
				RunID: m.ID, Kind: kind, Sequence: seq, Status: "running", StartedAt: &start,
			}
			if err := s.db.WithContext(ctx).Create(&step).Error; err != nil {
				return err
			}
			stepID = step.ID
			existing[seq] = &step
		}
		var stepErr error
		if kind == "build" {
			stepErr = s.executeBuildLoop(ctx, m)
		} else {
			stepErr = s.executeSingle(ctx, m, kind, &stepID)
		}
		fin := time.Now()
		stepStatus := "succeeded"
		summary := ""
		if stepErr != nil {
			stepStatus = "failed"
			summary = stepErr.Error()
		}
		_ = s.db.WithContext(ctx).Model(&models.RunStep{}).Where("id = ?", stepID).Updates(map[string]any{
			"status": stepStatus, "finished_at": fin, "output_summary": summary,
		}).Error
		if stepErr != nil {
			return stepErr
		}
	}
	return nil
}

// executeSingle 执行单阶段 Harness Run。
func (s *Service) executeSingle(ctx context.Context, m *models.Run, kind string, stepID *uuid.UUID) error {
	return s.runHarnessStage(ctx, m, kind, stepID, true)
}

// runHarnessStage 运行单个 Harness 阶段并持久化结果。
func (s *Service) runHarnessStage(ctx context.Context, m *models.Run, kind string, stepID *uuid.UUID, manageSandbox bool) error {
	if manageSandbox {
		if err := s.prepareRunSandbox(ctx, m, kind); err != nil {
			return err
		}
	}
	projectCode, err := s.workspace.ProjectWorkspaceKey(ctx, m.ProjectID)
	if err != nil {
		return err
	}
	if manageSandbox && s.shouldUseSharedLock(m) {
		unlock := s.sandboxLocks.acquire(projectCode, m.RepositoryID)
		defer unlock()
	}
	modelCfg, ok := s.runtimeCfg.AI.ActiveModel()
	if !ok || !config.ModelConfigured(modelCfg) {
		return errors.New("未配置模型：请在管理区域 → 系统配置中设置并启用默认模型")
	}
	msg := m.Title
	sandboxDir, err := s.sandboxDir(ctx, m)
	if err != nil {
		return err
	}
	docsRoot, err := s.workspace.DocsRoot(ctx, m.ProjectID)
	if err != nil {
		return err
	}
	planFilePath := s.harnessPlanFilePath(m.ProjectID, m.FilePath, kind)
	evalFilePath := s.resolveHarnessEvalPath(m.ProjectID, m.EvalFilePath)
	messages := BuildHarnessMessages(kind, msg, planFilePath, evalFilePath, sandboxDir, docsRoot)
	sessionsDir := storageProjectSessions(s.paths, projectCode)
	docSandbox, err := s.workspace.DocSandboxDir(ctx, m.ProjectID)
	if err != nil {
		return err
	}
	sink := s.compositeSink(m.ID, stepID)
	mcpPorts := mcpConfigsToPorts(s.runtimeCfg.MCP.Servers)
	result, runErr := s.runtime.Run(ctx, ports.RunRequest{
		RunID: m.ID.String(), Kind: kind, Messages: messages,
		SandboxDir: sandboxDir, ExtraSandboxDirs: []string{docSandbox},
		SessionsDir: sessionsDir,
		Model: ports.ModelConfig{
			BaseURL: modelCfg.BaseURL, APIKey: modelCfg.APIKey,
			Model: modelCfg.Model, MaxTokens: modelCfg.MaxTokens,
		},
		MCP: mcpPorts,
		Policy: ports.RuntimePolicy{
			AllowShell: s.runtimeCfg.AI.Security.AllowShell, AllowCommandMCP: s.runtimeCfg.AI.Security.AllowCommandMCP,
		},
	}, sink)
	if runErr != nil {
		return runErr
	}
	if result.Err != nil {
		return result.Err
	}
	s.indexHarnessOutputs(ctx, m, kind, docsRoot)
	return nil
}

// resolveHarnessDocPath 解析 Harness 文档逻辑路径为绝对路径。
func (s *Service) resolveHarnessDocPath(projectID uuid.UUID, filePath string) string {
	logical, err := s.workspace.SanitizeDocLogicalPath(filePath)
	if err != nil || logical == "" {
		return ""
	}
	abs, err := s.workspace.ResolveDocPath(projectID, logical)
	if err != nil {
		return ""
	}
	return abs
}

// resolveHarnessEvalPath 解析 Harness 评测文档路径。
func (s *Service) resolveHarnessEvalPath(projectID uuid.UUID, filePath string) string {
	return s.resolveHarnessDocPath(projectID, filePath)
}

// harnessPlanFilePath 解析 Harness 计划文件绝对路径。
func (s *Service) harnessPlanFilePath(projectID uuid.UUID, filePath, kind string) string {
	if abs := s.resolveHarnessDocPath(projectID, filePath); abs != "" {
		return abs
	}
	if kind != "plan" || strings.TrimSpace(filePath) != "" {
		return ""
	}
	logical := harness.PlanTargetPath("", time.Now())
	if abs, err := s.workspace.ResolveDocPath(projectID, logical); err == nil {
		return abs
	}
	return ""
}

// shouldNotifyRun 判断 Run 状态变更是否应发送通知。
func shouldNotifyRun(kind string) bool {
	switch kind {
	case "plan", "implement", "verify", "build":
		return true
	default:
		return false
	}
}

// IsStageKind 报告 kind 是否为 UI 中展示的 Harness 工作流阶段。
func IsStageKind(kind string) bool {
	return shouldNotifyRun(kind)
}

// indexHarnessOutputs 将 Harness 产出索引到 plans/artifacts 表。
func (s *Service) indexHarnessOutputs(ctx context.Context, m *models.Run, kind, docsRoot string) {
	switch kind {
	case "plan":
		if s.plans != nil {
			_ = s.plans.IndexAfterRun(ctx, m.ProjectID, m.RepositoryID, m.ID, m.FilePath, docsRoot)
		}
	case "verify", "build":
		if s.artifacts != nil {
			_ = s.artifacts.IndexAfterRun(ctx, m.ProjectID, m.RepositoryID, m.ID, m.FilePath, docsRoot)
		}
	}
}

// storageProjectSessions 返回项目会话审计目录路径。
func storageProjectSessions(paths storage.Paths, projectKey string) string {
	return storage.ProjectSessionsDir(paths, projectKey)
}

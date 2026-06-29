package run

import (
	"context"
	"errors"
	"fmt"
	"matrix/internal/ai/agent"
	"matrix/internal/ai/harness"
	"matrix/internal/ai/ports"
	"matrix/internal/ai/query"
	"matrix/internal/modules/eval"
	"matrix/internal/modules/run/view"
	"matrix/internal/platform/db/models"
	"matrix/internal/platform/logging"
	"matrix/internal/platform/storage"
	"os"
	"path/filepath"
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

// GetToolLog 返回工具 spill 日志内容。
func (s *Service) GetToolLog(ctx context.Context, runID uuid.UUID, toolUseID string) (string, error) {
	var m models.Run
	if err := s.db.WithContext(ctx).First(&m, "id = ?", runID).Error; err != nil {
		return "", err
	}
	matrixDir, err := s.workspace.MatrixDir(ctx, m.ProjectID, runID)
	if err != nil {
		return "", err
	}
	path := filepath.Join(matrixDir, "tool-outputs", sanitizeToolUseIDForLog(toolUseID)+".log")
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func sanitizeToolUseIDForLog(id string) string {
	out := make([]byte, 0, len(id))
	for i := 0; i < len(id); i++ {
		c := id[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' {
			out = append(out, c)
		} else {
			out = append(out, '_')
		}
	}
	if len(out) == 0 {
		return "tool"
	}
	return string(out)
}

func (s *Service) finishRunView(ctx context.Context, runID uuid.UUID, status, output, errMsg, mergeStatus string) error {
	if mergeStatus == "" {
		var m models.Run
		if err := s.db.WithContext(ctx).First(&m, "id = ?", runID).Error; err == nil {
			output = m.Output
			errMsg = m.ErrorMessage
			mergeStatus = m.MergeStatus
			if status == "" {
				status = m.Status
			}
		}
	}
	return s.viewStore.FinishRun(ctx, runID.String(), status, output, errMsg, mergeStatus)
}

// ExecuteRun 执行 Run 的 Harness 流水线。
func (s *Service) ExecuteRun(ctx context.Context, runID uuid.UUID) error {
	m, skip, err := s.loadRunForExecute(ctx, runID)
	if err != nil || skip {
		return err
	}
	if err := s.refreshAIRuntime(ctx); err != nil {
		logging.Agent("run: 刷新 AI 配置失败", "run_id", runID, "error", err.Error())
		return fmt.Errorf("刷新 AI 配置失败: %w", err)
	}
	runStart := time.Now()
	logging.Agent("run: 开始执行",
		"run_id", runID, "kind", m.Kind, "model_id", m.ModelID,
		"chat_session_id", m.ChatSessionID, "prior_status", m.Status,
	)
	now := time.Now()
	_ = s.db.WithContext(ctx).Model(&m).Updates(map[string]any{
		"status": "running", "started_at": now, "finished_at": nil, "error_message": "",
	}).Error
	if s.notifier != nil && shouldNotifyRun(m.Kind) {
		s.notifier.NotifyRunStatus(ctx, m.CreatedBy, m.ProjectID, runID, m.Kind, "running", m.Title)
	}
	_ = s.viewStore.BeginRun(ctx, runID.String(), m.ProjectID.String(), m.Kind, m.Kind)

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
	return s.finalizeRun(ctx, runID, m, runErr, runStart)
}

func (s *Service) loadRunForExecute(ctx context.Context, runID uuid.UUID) (models.Run, bool, error) {
	if err := ctx.Err(); err != nil {
		return models.Run{}, false, err
	}
	var m models.Run
	if err := s.db.WithContext(ctx).First(&m, "id = ?", runID).Error; err != nil {
		logging.Agent("run: 加载 Run 失败", "run_id", runID, "error", err.Error())
		return models.Run{}, false, err
	}
	switch m.Status {
	case "succeeded", "cancelled":
		logging.Agent("run: 跳过执行（已终态）",
			"run_id", runID, "kind", m.Kind, "status", m.Status,
			"output_len", len(m.Output), "error_message", m.ErrorMessage,
		)
		return m, true, nil
	}
	if err := ctx.Err(); err != nil {
		logging.Agent("run: 执行前 context 已取消", "run_id", runID, "error", err.Error())
		return m, false, err
	}
	return m, false, nil
}

func (s *Service) finalizeRun(ctx context.Context, runID uuid.UUID, m models.Run, runErr error, runStart time.Time) error {
	fin := time.Now()
	status := "succeeded"
	errMsg := ""
	if runErr != nil {
		if ctx.Err() != nil {
			status = "cancelled"
			errMsg = "任务已取消"
		} else {
			status = "failed"
			errMsg = runErr.Error()
		}
	}
	updates := map[string]any{
		"status": status, "finished_at": fin, "error_message": errMsg,
	}
	if ms := s.mergeStatusAfterRun(&m, status); ms != "" {
		updates["merge_status"] = ms
	}
	_ = s.db.WithContext(ctx).Model(&models.Run{}).Where("id = ?", runID).Updates(updates).Error
	if runErr == nil && status == "succeeded" && m.Kind == "chat" && m.ChatSessionID != nil && m.ChatUserMessageID != nil {
		s.ensureChatAssistantMessage(ctx, *m.ChatSessionID, *m.ChatUserMessageID, runID)
	}
	if runErr != nil && s.runtimeCfg.Run.CleanupOnFailure && m.SandboxPath != "" {
		_ = s.workspace.RemoveRunWorktree(ctx, m.ProjectID, m.RepositoryID, runID, m.RunBranch, m.SandboxPath)
		_ = s.db.WithContext(ctx).Model(&models.Run{}).Where("id = ?", runID).Updates(map[string]any{
			"sandbox_path": "", "run_branch": "",
		}).Error
	}
	if s.notifier != nil && shouldNotifyRun(m.Kind) {
		s.notifier.NotifyRunStatus(ctx, m.CreatedBy, m.ProjectID, runID, m.Kind, status, m.Title)
	}
	var finished models.Run
	_ = s.db.WithContext(ctx).First(&finished, "id = ?", runID).Error
	if err := s.finishRunView(ctx, runID, status, finished.Output, errMsg, finished.MergeStatus); err != nil {
		logging.Agent("run-view: 结束视图失败",
			"run_id", runID, "status", status, "error", err.Error(),
		)
	}
	logging.Agent("run: 执行结束",
		"run_id", runID, "kind", m.Kind, "status", status,
		"output_len", len(finished.Output),
		"error_message", errMsg,
		"duration_ms", time.Since(runStart).Milliseconds(),
	)
	return runErr
}

func (s *Service) ensureChatAssistantMessage(ctx context.Context, sessionID, userMessageID, runID uuid.UUID) {
	var count int64
	if err := s.db.WithContext(ctx).Model(&models.ChatMessage{}).Where("run_id = ?", runID).Count(&count).Error; err != nil {
		logging.Agent("run: 检查助手消息失败", "run_id", runID, "error", err.Error())
		return
	}
	if count > 0 {
		logging.Agent("run: 助手消息已存在", "run_id", runID)
		return
	}
	var m models.Run
	if err := s.db.WithContext(ctx).First(&m, "id = ?", runID).Error; err != nil {
		logging.Agent("run: 加载 Run 失败（写入助手消息）", "run_id", runID, "error", err.Error())
		return
	}
	assistantID, err := s.InsertChatAssistantMessage(ctx, sessionID, userMessageID, runID, m.Output)
	if err != nil {
		logging.Agent("run: 写入助手消息失败",
			"run_id", runID, "session_id", sessionID, "user_message_id", userMessageID,
			"error", err.Error(),
		)
		return
	}
	logging.Agent("run: 已写入助手消息",
		"run_id", runID, "assistant_message_id", assistantID, "output_len", len(m.Output),
	)
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
	return s.withSharedSandboxLock(ctx, m, func() error {
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
	})
}

func (s *Service) withSharedSandboxLock(ctx context.Context, m *models.Run, fn func() error) error {
	if !s.shouldUseSharedLock(m) {
		return fn()
	}
	projectCode, err := s.workspace.ProjectWorkspaceKey(ctx, m.ProjectID)
	if err != nil {
		return err
	}
	unlock := s.sandboxLocks.acquire(projectCode, m.RepositoryID)
	defer unlock()
	return fn()
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
		return errors.New("流水线未配置")
	}
	stages, err := s.pipelineStageKinds(ctx, m)
	if err != nil {
		return err
	}
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
		prev := existing[seq]
		if prev != nil && prev.Status == "succeeded" {
			continue
		}
		stepID, err := s.beginPipelineStep(ctx, m, seq, kind, prev, existing)
		if err != nil {
			return err
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
		s.viewStore.PublishStep(ctx, m.ID.String(), view.EventSTEPFinished, stepID.String(), kind, seq, stepStatus)
		if stepErr != nil {
			return stepErr
		}
	}
	return nil
}

func (s *Service) beginPipelineStep(
	ctx context.Context,
	m *models.Run,
	seq int,
	kind string,
	prev *models.RunStep,
	existing map[int]*models.RunStep,
) (uuid.UUID, error) {
	start := time.Now()
	if prev != nil {
		if err := s.db.WithContext(ctx).Model(prev).Updates(map[string]any{
			"status": "running", "started_at": start, "finished_at": nil, "output_summary": "",
		}).Error; err != nil {
			return uuid.Nil, err
		}
		s.viewStore.PublishStep(ctx, m.ID.String(), view.EventSTEPStarted, prev.ID.String(), kind, seq, "running")
		return prev.ID, nil
	}
	step := models.RunStep{
		RunID: m.ID, Kind: kind, Sequence: seq, Status: "running", StartedAt: &start,
	}
	if err := s.db.WithContext(ctx).Create(&step).Error; err != nil {
		return uuid.Nil, err
	}
	existing[seq] = &step
	s.viewStore.PublishStep(ctx, m.ID.String(), view.EventSTEPStarted, step.ID.String(), kind, seq, "running")
	return step.ID, nil
}

// executeSingle 执行单阶段 Harness Run。
func (s *Service) executeSingle(ctx context.Context, m *models.Run, kind string, stepID *uuid.UUID) error {
	return s.runHarnessStage(ctx, m, kind, stepID, true)
}

// runHarnessStage 运行单个 Harness 阶段并持久化结果。
func (s *Service) runHarnessStage(ctx context.Context, m *models.Run, kind string, _ *uuid.UUID, manageSandbox bool) error {
	if manageSandbox {
		if err := s.prepareRunSandbox(ctx, m, kind); err != nil {
			return err
		}
	}
	runStage := func() error {
		req, err := s.buildRunRequest(ctx, m, kind)
		if err != nil {
			return err
		}
		logging.Agent("run: Harness 阶段就绪",
			"run_id", m.ID, "kind", kind,
			"message_count", len(req.Messages),
			"sandbox_dir", req.SandboxDir,
			"model", req.Model.Model,
			"chat_session_id", m.ChatSessionID,
			"chat_user_message_id", m.ChatUserMessageID,
		)
		if len(req.Messages) == 0 {
			logging.Agent("run: LLM 输入消息为空", "run_id", m.ID, "kind", kind)
		}
		sink := s.viewStore.Sink(m.ID.String(), m.ProjectID.String())
		onSubagent := func(snap agent.Snapshot) {
			s.viewStore.OnSubagent(ctx, m.ID.String(), snap)
		}
		req.OnSubagentUpdate = onSubagent
		req.OnSubagentDone = onSubagent
		result, runErr := s.runtime.Run(ctx, req, sink)
		logging.Agent("run: runtime.Run 返回",
			"run_id", m.ID, "kind", kind,
			"run_err", runErr != nil,
			"result_err", result.Err,
			"output_len", len(runOutputFromResult(result)),
			"stop_reason", result.StopReason,
			"turn_count", result.TurnCount,
		)
		if runErr == nil && result.Err == nil {
			output := runOutputFromResult(result)
			if output != "" {
				_ = s.db.WithContext(ctx).Model(&models.Run{}).Where("id = ?", m.ID).
					Update("output", output).Error
			}
		}
		if runErr != nil {
			return runErr
		}
		if result.Err != nil {
			return result.Err
		}
		docsRoot, err := s.workspace.DocsRoot(ctx, m.ProjectID)
		if err != nil {
			return err
		}
		s.indexHarnessOutputs(ctx, m, kind, docsRoot)
		return nil
	}
	if manageSandbox {
		return s.withSharedSandboxLock(ctx, m, runStage)
	}
	return runStage()
}

func (s *Service) buildRunRequest(ctx context.Context, m *models.Run, kind string) (ports.RunRequest, error) {
	projectCode, err := s.workspace.ProjectWorkspaceKey(ctx, m.ProjectID)
	if err != nil {
		return ports.RunRequest{}, err
	}
	modelCfg, _, err := s.runtimeCfg.AI.ResolveModel(m.ModelID)
	if err != nil {
		return ports.RunRequest{}, err
	}
	sandboxDir, err := s.sandboxDir(ctx, m)
	if err != nil {
		return ports.RunRequest{}, err
	}
	docsRoot, err := s.workspace.DocsRoot(ctx, m.ProjectID)
	if err != nil {
		return ports.RunRequest{}, err
	}
	planAbsPath := s.harnessPlanFilePath(m.ProjectID, m.FilePath, kind)
	evalFilePath := s.resolveHarnessDocPath(m.ProjectID, m.EvalFilePath)
	var messages []query.Message
	if kind == "chat" {
		messages, err = s.buildChatRunMessages(ctx, m)
		if err != nil {
			return ports.RunRequest{}, err
		}
	} else {
		messages = BuildHarnessMessages(kind, m.Title, m.FilePath, planAbsPath, evalFilePath, sandboxDir, docsRoot)
	}
	docSandbox, err := s.workspace.DocSandboxDir(ctx, m.ProjectID)
	if err != nil {
		return ports.RunRequest{}, err
	}
	matrixDir, err := s.workspace.MatrixDir(ctx, m.ProjectID, m.ID)
	if err != nil {
		return ports.RunRequest{}, err
	}
	allowCommandMCP := s.runtimeCfg.AI.Security.AllowCommandMCP
	return ports.RunRequest{
		RunID: m.ID.String(), Kind: kind, Messages: messages,
		SandboxDir: sandboxDir, ExtraSandboxDirs: []string{docSandbox}, MatrixDir: matrixDir,
		SessionsDir: storageProjectSessions(s.paths, projectCode),
		Model: ports.ModelConfig{
			BaseURL: modelCfg.BaseURL, APIKey: modelCfg.APIKey,
			Model: modelCfg.Model, MaxTokens: modelCfg.MaxTokens,
		},
		MCP: mcpConfigsToPorts(s.runtimeCfg.MCP.Servers, allowCommandMCP),
		Policy: ports.RuntimePolicy{
			AllowShell: s.runtimeCfg.AI.Security.AllowShell, AllowCommandMCP: allowCommandMCP,
		},
	}, nil
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

// shouldNotifyRun 判断 Run 状态变更是否应发送通知，也是 UI 阶段列表的 kind 白名单。
func shouldNotifyRun(kind string) bool {
	switch kind {
	case "plan", "implement", "verify", "build":
		return true
	default:
		return false
	}
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

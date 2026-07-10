package run

import (
	"context"
	"errors"
	"fmt"
	"matrix/internal/ai/agent"
	"matrix/internal/ai/harness"
	"matrix/internal/ai/query"
	"matrix/internal/ai/tools"
	"matrix/internal/modules/eval"
	"matrix/internal/modules/run/view"
	"matrix/internal/modules/workspace"
	"matrix/internal/platform/db/models"
	"matrix/internal/platform/logging"
	"matrix/internal/platform/storage"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

// GetAuditForProject 返回项目内 Run 审计日志。
func (s *Service) GetAuditForProject(ctx context.Context, projectID, runID uuid.UUID) (string, error) {
	m, err := s.loadProjectRun(ctx, projectID, runID)
	if err != nil {
		return "", err
	}
	return s.readAuditFile(*m)
}

func (s *Service) readAuditFile(m models.Run) (string, error) {
	if m.AuditPath == "" {
		return "", nil
	}
	b, err := os.ReadFile(m.AuditPath)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// GetToolLogForProject 返回项目内 Run 工具 spill 日志。
func (s *Service) GetToolLogForProject(ctx context.Context, projectID, runID uuid.UUID, toolUseID string) (string, error) {
	m, err := s.loadProjectRun(ctx, projectID, runID)
	if err != nil {
		return "", err
	}
	return s.readToolLog(ctx, *m, toolUseID)
}

func (s *Service) readToolLog(ctx context.Context, m models.Run, toolUseID string) (string, error) {
	matrixDir, err := s.workspace.MatrixDir(ctx, m.ProjectID, m.ID)
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

func (s *Service) finishRunView(ctx context.Context, runID uuid.UUID, status, output, errMsg string) error {
	return s.viewStore.FinishRun(ctx, runID.String(), status, output, errMsg)
}

// ExecuteRun 执行 Run 全生命周期。
func (s *Service) ExecuteRun(ctx context.Context, runID uuid.UUID) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	row, err := s.stores.Run.GetByID(ctx, runID)
	if err != nil {
		logging.Agent("run: 加载 Run 失败", "run_id", runID, "error", err.Error())
		return err
	}
	m := *row
	if m.Status == "succeeded" || m.Status == "cancelled" {
		logging.Agent("run: 跳过执行（已终态）",
			"run_id", runID, "kind", m.Kind, "status", m.Status,
			"output_len", len(m.Output), "error_message", m.ErrorMessage,
		)
		return nil
	}
	if err := ctx.Err(); err != nil {
		logging.Agent("run: 执行前 context 已取消", "run_id", runID, "error", err.Error())
		return err
	}

	runStart := time.Now()
	logging.Agent("run: 开始执行",
		"run_id", runID, "kind", m.Kind, "model_id", m.ModelID,
		"chat_session_id", m.ChatSessionID, "prior_status", m.Status,
	)
	now := time.Now()
	if err := s.stores.Run.MarkRunning(ctx, runID, now); err != nil {
		logging.Agent("run: 更新运行状态失败", "run_id", runID, "error", err.Error())
		return err
	}
	if s.notifier != nil && Kind(m.Kind).IsHarness() {
		s.notifier.NotifyRunStatus(ctx, m.CreatedBy, m.ProjectID, runID, m.Kind, "running", m.Title)
	}
	if err := s.viewStore.BeginRun(ctx, runID.String(), m.ProjectID.String(), m.Kind, m.Kind); err != nil {
		return err
	}

	// sandbox
	if err := s.prepareRunRepo(ctx, &m); err != nil {
		return s.finalizeRun(ctx, runID, m, err, runStart)
	}

	// runBody
	runErr := s.runBodyByKind(ctx, &m)
	if ctx.Err() != nil {
		runErr = ctx.Err()
	}

	// finalize
	return s.finalizeRun(ctx, runID, m, runErr, runStart)
}

// runBodyByKind 按 Run 类型分发执行体，所有 AI 阶段收敛到 executeRunStage。
func (s *Service) runBodyByKind(ctx context.Context, m *models.Run) error {
	kind := Kind(m.Kind)
	switch kind {
	case KindBuild:
		return s.executeBuildLoop(ctx, m)
	default:
		var sess *agentSession
		return s.executeRunStage(ctx, m, &kind, &sess)
	}
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
	finished, err := s.stores.Run.Finalize(ctx, runID, status, errMsg, fin)
	if err != nil {
		logging.Agent("run: 更新终态失败", "run_id", runID, "status", status, "error", err.Error())
		return errors.Join(runErr, err)
	}
	if runErr == nil && Kind(m.Kind) == KindChat && m.ChatSessionID != nil && m.ChatUserMessageID != nil {
		s.ensureChatAssistantMessage(ctx, *m.ChatSessionID, *m.ChatUserMessageID, runID)
	}
	if s.notifier != nil && Kind(m.Kind).IsHarness() {
		s.notifier.NotifyRunStatus(ctx, m.CreatedBy, m.ProjectID, runID, m.Kind, status, m.Title)
	}
	if err := s.finishRunView(ctx, runID, status, finished.Output, errMsg); err != nil {
		logging.Agent("run-view: 结束视图失败",
			"run_id", runID, "status", status, "error", err.Error(),
		)
	}
	durationMs := time.Since(runStart).Milliseconds()
	logging.Agent("run: 执行结束",
		"run_id", runID, "kind", m.Kind, "status", status,
		"output_len", len(finished.Output),
		"error_message", errMsg,
		"duration_ms", durationMs,
	)
	if status == "failed" && strings.Contains(errMsg, "llm: 服务端返回") {
		logging.Agent("run: LLM 端点拒绝",
			"run_id", runID, "kind", m.Kind,
			"error_message", errMsg,
			"duration_ms", durationMs,
		)
	}
	return runErr
}

func (s *Service) ensureChatAssistantMessage(ctx context.Context, sessionID, userMessageID, runID uuid.UUID) {
	has, err := s.stores.Run.HasAssistantMessage(ctx, runID)
	if err != nil {
		logging.Agent("run: 检查助手消息失败", "run_id", runID, "error", err.Error())
		return
	}
	if has {
		logging.Agent("run: 助手消息已存在", "run_id", runID)
		return
	}
	output, err := s.stores.Run.GetOutput(ctx, runID)
	if err != nil {
		logging.Agent("run: 加载 Run 失败（写入助手消息）", "run_id", runID, "error", err.Error())
		return
	}
	assistantID, err := s.InsertChatAssistantMessage(ctx, sessionID, userMessageID, runID, output)
	if err != nil {
		logging.Agent("run: 写入助手消息失败",
			"run_id", runID, "session_id", sessionID, "user_message_id", userMessageID,
			"error", err.Error(),
		)
		return
	}
	logging.Agent("run: 已写入助手消息",
		"run_id", runID, "assistant_message_id", assistantID, "output_len", len(output),
	)
}

// prepareRunRepo 为 Run 准备 Git 沙箱：implement/verify 通过 copyRepo 复用同 plan 来源 repo（implement 无历史时 clone），其余阶段独立 clone。
func (s *Service) prepareRunRepo(ctx context.Context, m *models.Run) error {
	if m.SandboxPath != "" {
		return nil
	}
	var sandboxPath string
	var err error
	switch Kind(m.Kind) {
	case KindVerify:
		var sourceRunID uuid.UUID
		sandboxPath, sourceRunID, _, err = s.copyRepo(ctx, m, copyRepoOpts{
			stage:         "verify",
			sourceKinds:   verifySourceKinds,
			requireSource: true,
			notFoundHint:  "implement",
		})
		if err != nil {
			return err
		}
		m.SourceSandboxRunID = sourceRunID
	case KindImplement:
		var sourceRunID uuid.UUID
		var copied bool
		sandboxPath, sourceRunID, copied, err = s.copyRepo(ctx, m, copyRepoOpts{
			stage:         "implement",
			sourceKinds:   codeSandboxKinds,
			requireSource: false,
			notFoundHint:  "实现",
		})
		if err != nil {
			return err
		}
		if copied {
			m.SourceSandboxRunID = sourceRunID
		} else {
			sandboxPath, err = s.cloneRunRepo(ctx, m)
			if err != nil {
				return err
			}
		}
	default:
		sandboxPath, err = s.cloneRunRepo(ctx, m)
		if err != nil {
			return err
		}
	}
	m.SandboxPath = sandboxPath
	return s.stores.Run.UpdateSandboxPath(ctx, m.ID, sandboxPath)
}

func (s *Service) cloneRunRepo(ctx context.Context, m *models.Run) (string, error) {
	logging.Agent("run: 正在准备仓库沙箱", "run_id", m.ID, "kind", m.Kind)
	s.viewStore.SetStatusLabel(ctx, m.ID.String(), "正在克隆仓库…")
	sandboxPath, err := s.workspace.CreateRunRepo(ctx, m.ProjectID, m.RepositoryID, m.ID)
	if err != nil {
		if workspace.IsSourceFetchError(err) {
			s.viewStore.SetStatusLabel(ctx, m.ID.String(), view.FormatUserRunError(err.Error()))
		}
		return "", err
	}
	return sandboxPath, nil
}

const buildMaxRounds = 5

// executeBuildLoop 执行 build 阶段的闭环迭代（共享 agentSession）。
func (s *Service) executeBuildLoop(ctx context.Context, m *models.Run) error {
	var sess *agentSession
	for round := 1; round <= buildMaxRounds; round++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		implKind := KindImplement
		if err := s.executeRunStage(ctx, m, &implKind, &sess); err != nil {
			return fmt.Errorf("构建第 %d 轮编码: %w", round, err)
		}
		verifyKind := KindVerify
		if err := s.executeRunStage(ctx, m, &verifyKind, &sess); err != nil {
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

// executeRunStage 执行单次 AI 阶段；sess 非 nil 时复用会话基础设施（build 多轮）。
func (s *Service) executeRunStage(ctx context.Context, m *models.Run, kind *Kind, sess **agentSession) error {
	if m.SandboxPath == "" {
		return fmt.Errorf("run 仓库未就绪")
	}

	projectCode, err := s.workspace.ProjectWorkspaceKey(ctx, m.ProjectID)
	if err != nil {
		return err
	}

	if *sess == nil {
		aiCfg, err := s.settings.LoadAIConfig(ctx)
		if err != nil {
			return fmt.Errorf("加载 AI 配置失败: %w", err)
		}
		mcpCfg, err := s.settings.LoadMCPConfig(ctx)
		if err != nil {
			return fmt.Errorf("加载 MCP 配置失败: %w", err)
		}
		modelSpec, profile, err := aiCfg.ResolveModel(m.ModelID)
		if err != nil {
			return err
		}
		if modelSpec.APIKey == "" {
			return errors.New("未配置 API Key")
		}
		runID := m.ID.String()
		sink := s.viewStore.Sink(runID, m.ProjectID.String())
		onSubagent := func(snap agent.Snapshot) {
			s.viewStore.OnSubagent(ctx, runID, snap)
		}
		sessionsDir := storageProjectSessions(s.paths, projectCode)
		*sess = s.newAgentSession(aiCfg, mcpCfg, modelSpec, profile, m.SandboxPath, sessionsDir, runID, sink, onSubagent)
	}
	session := *sess

	docsRoot, err := s.workspace.DocsRoot(ctx, m.ProjectID)
	if err != nil {
		return err
	}
	planAbsPath := s.harnessPlanFilePath(m.ProjectID, m.FilePath, kind)
	evalFilePath := s.harnessEvalAbsPath(m.ProjectID, docsRoot, m, kind)

	var messages []query.Message
	if *kind == KindChat {
		messages, err = s.buildChatRunMessages(ctx, m)
		if err != nil {
			return err
		}
	} else {
		messages = harness.BuildMessages(
			kind.String(), m.Title, m.FilePath, planAbsPath, evalFilePath,
			m.SandboxPath, docsRoot, m.SourceSandboxRunID.String(),
		)
	}
	if len(messages) == 0 {
		return errors.New("消息不能为空")
	}

	logging.Agent("run: AI 阶段就绪",
		"run_id", m.ID, "kind", kind,
		"message_count", len(messages),
		"sandbox_dir", m.SandboxPath,
		"model", session.model,
	)

	docSandbox, err := s.workspace.DocSandboxDir(ctx, m.ProjectID)
	if err != nil {
		return err
	}
	matrixDir, err := s.workspace.MatrixDir(ctx, m.ProjectID, m.ID)
	if err != nil {
		return err
	}

	runCtx, cleanup := s.attachRunCancel(ctx, m.ID.String(), m.SandboxPath, []string{docSandbox}, matrixDir)
	defer cleanup()

	start := time.Now()
	result, runErr := session.runPhase(runCtx, *kind, messages)
	durationMs := time.Since(start).Milliseconds()
	output := runOutput(result)
	logging.Agent("run: AI 阶段结束",
		"run_id", m.ID, "kind", kind,
		"stop_reason", result.StopReason,
		"turn_count", result.TurnCount,
		"output_len", len(output),
		"duration_ms", durationMs,
		"has_error", runErr != nil,
	)
	if runErr != nil {
		logging.Agent("run: LLM 调用失败",
			"run_id", m.ID, "stop_reason", result.StopReason,
			"error", runErr.Error(), "duration_ms", durationMs,
		)
		return runErr
	}
	if output != "" {
		_ = s.stores.Run.UpdateOutput(ctx, m.ID, output)
	}
	s.indexHarnessOutputs(ctx, m, kind, docsRoot)
	return nil
}

// harnessEvalAbsPath 返回 Harness 阶段使用的评测报告绝对路径（按 PLAN 名自动查找最新 EVAL）。
func (s *Service) harnessEvalAbsPath(projectID uuid.UUID, docsRoot string, m *models.Run, kind *Kind) string {
	switch *kind {
	case KindImplement:
		if strings.TrimSpace(m.FilePath) == "" {
			return ""
		}
		evalRel, _, err := eval.LatestEval(docsRoot, m.FilePath)
		if err != nil {
			return ""
		}
		return s.resolveHarnessDocPath(projectID, evalRel)
	default:
		return ""
	}
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
func (s *Service) harnessPlanFilePath(projectID uuid.UUID, filePath string, kind *Kind) string {
	if abs := s.resolveHarnessDocPath(projectID, filePath); abs != "" {
		return abs
	}
	if *kind != KindPlan || strings.TrimSpace(filePath) != "" {
		return ""
	}
	logical := harness.PlanTargetPath("", time.Now())
	if abs, err := s.workspace.ResolveDocPath(projectID, logical); err == nil {
		return abs
	}
	return ""
}

// indexHarnessOutputs 将 Harness 产出索引到 plans/artifacts 表。
func (s *Service) indexHarnessOutputs(ctx context.Context, m *models.Run, kind *Kind, docsRoot string) {
	switch *kind {
	case KindPlan:
		if s.plans != nil {
			_ = s.plans.IndexAfterRun(ctx, m.ProjectID, m.RepositoryID, m.ID, m.FilePath, docsRoot)
		}
	case KindVerify:
		if s.artifacts != nil {
			_ = s.artifacts.IndexAfterRun(ctx, m.ProjectID, m.RepositoryID, m.ID, m.FilePath, docsRoot)
		}
	}
}

func runOutput(result query.Result) string {
	if result.Answer != "" {
		return result.Answer
	}
	if len(result.Messages) > 0 {
		return result.Messages[len(result.Messages)-1].Content
	}
	return ""
}

// storageProjectSessions 返回项目会话审计目录路径。
func storageProjectSessions(paths storage.Paths, projectKey string) string {
	return storage.ProjectSessionsDir(paths, projectKey)
}

func (s *Service) cancelAgentRun(runID string) {
	s.runCancelMu.Lock()
	cancel, ok := s.runCancels[runID]
	s.runCancelMu.Unlock()
	if ok && cancel != nil {
		cancel()
	}
}

func (s *Service) attachRunCancel(ctx context.Context, runID, sandboxDir string, extraSandbox []string, matrixDir string) (context.Context, func()) {
	runCtx, cancel := context.WithCancel(ctx)
	runCtx = logging.With(runCtx, logging.Fields{
		logging.FieldRunID:     runID,
		logging.FieldSessionID: runID,
	})
	runCtx = tools.WithSandbox(runCtx, sandboxDir)
	if len(extraSandbox) > 0 {
		runCtx = tools.WithExtraSandboxRoots(runCtx, extraSandbox)
	}
	if matrixDir != "" {
		runCtx = tools.WithMatrixDir(runCtx, matrixDir)
	}
	s.runCancelMu.Lock()
	s.runCancels[runID] = cancel
	s.runCancelMu.Unlock()
	return runCtx, func() {
		s.runCancelMu.Lock()
		delete(s.runCancels, runID)
		s.runCancelMu.Unlock()
		cancel()
	}
}

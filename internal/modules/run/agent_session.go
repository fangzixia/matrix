package run

import (
	"context"
	"path/filepath"
	"time"

	ai "matrix/ai/sdk"
	"matrix/internal/modules/run/audit"
	"matrix/internal/modules/run/harness"
	"matrix/internal/platform/config"
	"matrix/internal/platform/logging"
)

// agentSession 复用 MCP、Registry 与 Coordinator 基础设施，跨 build 多轮阶段共享。
type agentSession struct {
	client     *ai.Client
	model      string
	maxTokens  int
	allowShell bool
	ctxPolicy  ai.ContextPolicy

	mcpMgr     *ai.MCPManager
	registry   *ai.AgentRegistry
	coordAsync *ai.AsyncSupport
	workerRun  *ai.RunControl
	sidechain  *ai.AgentSidechainWriter

	sandboxDir  string
	sessionsDir string
	jobID       string
	threadID    string
	viewSink    ai.Sink
	onSubagent  func(ai.AgentSnapshot)
}

func (s *Service) newAgentSession(
	aiCfg config.AIConfig,
	mcpCfg config.MCPConfig,
	modelSpec config.ModelSpec,
	profile config.ModelProfile,
	sandboxDir, sessionsDir, jobID, threadID string,
	viewSink ai.Sink,
	onSubagent func(ai.AgentSnapshot),
) *agentSession {
	client := ai.NewClient(modelSpec.BaseURL, modelSpec.APIKey)
	client.ModelName = profile.Name
	subagentsDir := filepath.Join(filepath.Dir(sessionsDir), "subagents")
	return &agentSession{
		client:     client,
		model:      modelSpec.Model,
		maxTokens:  modelSpec.MaxTokens,
		allowShell: aiCfg.Security.AllowShell,
		ctxPolicy: ai.ContextPolicy{
			AutoCompactThreshold: aiCfg.Context.AutoCompactThreshold,
			KeepRecentMessages:   aiCfg.Context.KeepRecentMessages,
		},
		mcpMgr:      newMCPManager(mcpCfg.Servers, aiCfg.Security.AllowCommandMCP),
		registry:    ai.NewAgentRegistry(),
		coordAsync:  ai.NewAsyncSupport(),
		workerRun:   ai.NewRunControl(),
		sidechain:   ai.NewAgentSidechainWriter(subagentsDir),
		sandboxDir:  sandboxDir,
		sessionsDir: sessionsDir,
		jobID:       jobID,
		threadID:    threadID,
		viewSink:    viewSink,
		onSubagent:  onSubagent,
	}
}

func (a *agentSession) runPhase(ctx context.Context, kind Kind, messages []ai.Message) (ai.Result, error) {
	aguiRunID := ai.NewRunID()
	streamSink, closeSink := ai.BuildCoalescedSink(a.viewSink, 100*time.Millisecond, 200*time.Millisecond)
	defer closeSink()

	auditWriter := audit.NewWriter(a.sessionsDir, a.sandboxDir, aguiRunID)
	hub := ai.NewStreamHub(a.threadID, aguiRunID, a.registry, a.sidechain, streamSink, nil, a.onSubagent, a.onSubagent)
	hub.Audit = auditWriter

	qCfg := a.buildQueryConfig(kind, messages, a.mcpMgr, hub, auditWriter, aguiRunID)
	a.workerRun.SetParent(ctx)
	defer a.workerRun.SetParent(context.Background())

	start := time.Now()
	result := ai.RunSession(ctx, qCfg, streamSink)
	_ = auditWriter.Close(audit.SessionMeta{
		StopReason: string(result.StopReason),
		TurnCount:  result.TurnCount,
		DurationMs: time.Since(start).Milliseconds(),
	})
	return result, result.Err
}

func (a *agentSession) buildQueryConfig(
	kind Kind,
	messages []ai.Message,
	mcpMgr *ai.MCPManager,
	hub *ai.StreamHub,
	auditWriter *audit.Writer,
	aguiRunID string,
) ai.Config {
	reg := ai.DefaultRegistry()
	if !a.allowShell {
		reg = ai.RegistryWithoutShell(nil)
	}
	_ = ai.RegisterMCPTools(reg, mcpMgr)
	workerOnly := ai.CloneWorkerRegistry(reg)
	coordCfg := ai.CoordinatorConfig{
		LLM:           a.client,
		Model:         a.model,
		AgentRegistry: a.registry,
		ToolRegistry:  workerOnly,
		CanUseTool:    func(string, map[string]any) bool { return true },
		MaxTurns:      200,
		MaxTokens:     a.maxTokens,
		ContextPolicy: a.ctxPolicy,
		Async:         a.coordAsync,
		RunControl:    a.workerRun,
		StreamHub:     hub,
		ThreadID:      a.threadID,
		RunID:         aguiRunID,
		SessionID:     aguiRunID,
		Audit:         auditWriter,
		SandboxDir:    a.sandboxDir,
		WorkerUserMessageFormat: func(p string) string {
			return harness.FormatWorkspaceUserMessage(a.sandboxDir, "", p, "")
		},
	}
	parentReg := ai.NewParentRegistry(coordCfg)
	asyncResults, hasPending := a.coordAsync.QueryConfigFields()
	prompt := ai.BuildParentSystemPrompt(workerOnly.Names(), mcpMgr.Names())
	if kind == KindVerify {
		prompt += "\n\n" + harness.VerifyCoordinatorSupplement
	}
	logging.Agent("run: 构建 Query 配置", "job_id", a.jobID, "run_id", aguiRunID, "sandbox", a.sandboxDir)
	cfg := ai.QueryConfigFromCoordinator(coordCfg, ai.CoordinatorQueryConfigOverrides{
		SystemPrompt:    prompt,
		Registry:        parentReg,
		AsyncResults:    asyncResults,
		HasPendingAsync: hasPending,
		InitialMessages: append([]ai.Message(nil), messages...),
	})
	cfg.ThreadID = a.threadID
	cfg.RunID = aguiRunID
	cfg.ParentRunID = a.jobID
	cfg.SessionID = aguiRunID
	return cfg
}

func newMCPManager(servers map[string]config.MCPServerConfig, allowCommandMCP bool) *ai.MCPManager {
	m := ai.NewMCPManager()
	cfgs := map[string]ai.MCPServerConfig{}
	for name, srv := range servers {
		if srv.Disabled {
			continue
		}
		if !allowCommandMCP && srv.Command != "" {
			continue
		}
		cfgs[name] = ai.MCPServerConfig{
			Command: srv.Command, Args: srv.Args, URL: srv.URL,
			Headers: srv.Headers, Env: srv.Env, Disabled: srv.Disabled,
		}
	}
	m.UpdateConfigs(cfgs)
	return m
}

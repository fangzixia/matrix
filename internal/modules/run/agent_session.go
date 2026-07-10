package run

import (
	"context"
	"path/filepath"
	"time"

	"matrix/internal/ai/agent"
	"matrix/internal/ai/audit"
	"matrix/internal/ai/coordinator"
	"matrix/internal/ai/harness"
	"matrix/internal/ai/llm"
	"matrix/internal/ai/mcp"
	"matrix/internal/ai/query"
	"matrix/internal/ai/stream"
	"matrix/internal/ai/tools"
	"matrix/internal/platform/config"
	"matrix/internal/platform/logging"
)

// agentSession 复用 MCP、Registry 与 Coordinator 基础设施，跨 build 多轮阶段共享。
type agentSession struct {
	client     *llm.Client
	model      string
	maxTokens  int
	allowShell bool
	ctxPolicy  query.ContextPolicy

	mcpMgr     *mcp.Manager
	registry   *agent.Registry
	coordAsync *coordinator.AsyncSupport
	workerRun  *coordinator.RunControl
	sidechain  *agent.SidechainWriter

	sandboxDir  string
	sessionsDir string
	sessionID   string
	viewSink    stream.Sink
	onSubagent  func(agent.Snapshot)
}

func (s *Service) newAgentSession(
	aiCfg config.AIConfig,
	mcpCfg config.MCPConfig,
	modelSpec config.ModelSpec,
	profile config.ModelProfile,
	sandboxDir, sessionsDir, sessionID string,
	viewSink stream.Sink,
	onSubagent func(agent.Snapshot),
) *agentSession {
	client := llm.NewClient(modelSpec.BaseURL, modelSpec.APIKey)
	client.ModelName = profile.Name
	subagentsDir := filepath.Join(filepath.Dir(sessionsDir), "subagents")
	return &agentSession{
		client:     client,
		model:      modelSpec.Model,
		maxTokens:  modelSpec.MaxTokens,
		allowShell: aiCfg.Security.AllowShell,
		ctxPolicy: query.ContextPolicy{
			AutoCompactThreshold: aiCfg.Context.AutoCompactThreshold,
			KeepRecentMessages:   aiCfg.Context.KeepRecentMessages,
		},
		mcpMgr:      newMCPManager(mcpCfg.Servers, aiCfg.Security.AllowCommandMCP),
		registry:    agent.NewRegistry(),
		coordAsync:  coordinator.NewAsyncSupport(),
		workerRun:   coordinator.NewRunControl(),
		sidechain:   agent.NewSidechainWriter(subagentsDir),
		sandboxDir:  sandboxDir,
		sessionsDir: sessionsDir,
		sessionID:   sessionID,
		viewSink:    viewSink,
		onSubagent:  onSubagent,
	}
}

func (a *agentSession) runPhase(ctx context.Context, kind Kind, messages []query.Message) (query.Result, error) {
	streamSink, closeSink := buildStreamingSink(a.viewSink, a.sessionID)
	defer closeSink()

	auditWriter := audit.NewWriter(a.sessionsDir, a.sandboxDir, a.sessionID)
	hub := coordinator.NewStreamHub(a.sessionID, a.registry, a.sidechain, streamSink, nil, a.onSubagent, a.onSubagent)
	hub.Audit = auditWriter

	qCfg := a.buildQueryConfig(kind, messages, a.mcpMgr, hub, auditWriter)
	a.workerRun.SetParent(ctx)
	defer a.workerRun.SetParent(context.Background())

	start := time.Now()
	result := query.RunSession(ctx, qCfg, streamSink)
	_ = auditWriter.Close(audit.SessionMeta{
		StopReason: string(result.StopReason),
		TurnCount:  result.TurnCount,
		DurationMs: time.Since(start).Milliseconds(),
	})
	return result, result.Err
}

func (a *agentSession) buildQueryConfig(
	kind Kind,
	messages []query.Message,
	mcpMgr *mcp.Manager,
	hub *coordinator.StreamHub,
	auditWriter *audit.Writer,
) query.Config {
	reg := tools.DefaultRegistry()
	if !a.allowShell {
		reg = tools.RegistryWithoutShell(nil)
	}
	_ = tools.RegisterMCPTools(reg, mcpMgr)
	workerOnly := coordinator.CloneWorkerRegistry(reg)
	coordCfg := coordinator.Config{
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
		SessionID:     a.sessionID,
		Audit:         auditWriter,
		SandboxDir:    a.sandboxDir,
	}
	parentReg := coordinator.NewParentRegistry(coordCfg)
	asyncResults, hasPending := a.coordAsync.QueryConfigFields()
	prompt := coordinator.BuildParentSystemPrompt(workerOnly.Names(), mcpMgr.Names())
	if kind == KindVerify {
		prompt += "\n\n" + harness.VerifyCoordinatorSupplement
	}
	logging.Agent("run: 构建 Query 配置", "session_id", a.sessionID, "sandbox", a.sandboxDir)
	return coordinator.QueryConfigFromCoordinator(coordCfg, coordinator.QueryConfigOverrides{
		SystemPrompt:    prompt,
		Registry:        parentReg,
		AsyncResults:    asyncResults,
		HasPendingAsync: hasPending,
		InitialMessages: append([]query.Message(nil), messages...),
	})
}

func buildStreamingSink(base stream.Sink, runID string) (stream.Sink, func()) {
	coalescedText := stream.NewCoalesceSink(base, runID, 100*time.Millisecond)
	coalesced := stream.NewOutputCoalesceSink(coalescedText, runID, 200*time.Millisecond)
	return coalesced, func() {
		coalesced.Close()
		coalescedText.Close()
	}
}

func newMCPManager(servers map[string]config.MCPServerConfig, allowCommandMCP bool) *mcp.Manager {
	m := mcp.NewManager()
	cfgs := map[string]mcp.ServerConfig{}
	for name, srv := range servers {
		if srv.Disabled {
			continue
		}
		if !allowCommandMCP && srv.Command != "" {
			continue
		}
		cfgs[name] = mcp.ServerConfig{
			Command: srv.Command, Args: srv.Args, URL: srv.URL,
			Headers: srv.Headers, Env: srv.Env, Disabled: srv.Disabled,
		}
	}
	m.UpdateConfigs(cfgs)
	return m
}

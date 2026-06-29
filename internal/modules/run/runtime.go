package run

import (
	"context"
	"errors"
	"matrix/internal/ai/agent"
	"matrix/internal/ai/audit"
	"matrix/internal/ai/coordinator"
	"matrix/internal/ai/llm"
	"matrix/internal/ai/mcp"
	"matrix/internal/ai/ports"
	"matrix/internal/ai/query"
	"matrix/internal/ai/stream"
	"matrix/internal/ai/tools"
	"matrix/internal/platform/config"
	"matrix/internal/platform/logging"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Runtime 封装 AI Agent 会话执行：LLM 调用、MCP 工具与取消控制。
type Runtime struct {
	runtimeCfg *config.RuntimeConfig
	mu         sync.Mutex
	runCancels map[string]context.CancelFunc
}

// NewRuntime 创建 AI 运行时实例。
func NewRuntime(runtime *config.RuntimeConfig) *Runtime {
	return &Runtime{
		runtimeCfg: runtime,
		runCancels: make(map[string]context.CancelFunc),
	}
}

// Close 释放运行时持有的资源。
func (r *Runtime) Close() {}

// Cancel 取消指定 Run 的执行。
func (r *Runtime) Cancel(runID string) error {
	r.mu.Lock()
	cancel, ok := r.runCancels[runID]
	r.mu.Unlock()
	if !ok || cancel == nil {
		return nil
	}
	cancel()
	return nil
}

// Run 在沙箱中执行一次 AI Agent 会话并流式输出事件。
func (r *Runtime) Run(ctx context.Context, req ports.RunRequest, sink stream.Sink) (ports.RunResult, error) {
	logging.Agent("run: runtime.Run 开始",
		"run_id", req.RunID, "kind", req.Kind,
		"message_count", len(req.Messages),
		"sandbox_dir", req.SandboxDir,
		"model", req.Model.Model,
	)
	if err := validateRunRequest(req); err != nil {
		return ports.RunResult{}, err
	}
	runID := req.RunID
	if runID == "" {
		runID = uuid.NewString()
	}
	runCtx, cleanup := r.attachRunCancel(ctx, runID, req.SandboxDir, req.ExtraSandboxDirs)
	defer cleanup()
	streamSink, closeSink := r.buildStreamingSink(sink, runID)
	defer closeSink()
	return r.runTAORSession(runCtx, req, runID, streamSink)
}

func validateRunRequest(req ports.RunRequest) error {
	if req.Model.APIKey == "" {
		logging.Agent("run: runtime.Run 拒绝（无 API Key）", "run_id", req.RunID)
		return errors.New("未配置 API Key")
	}
	if len(req.Messages) == 0 {
		logging.Agent("run: runtime.Run 拒绝（消息为空）", "run_id", req.RunID, "kind", req.Kind)
		return errors.New("消息不能为空")
	}
	if req.SandboxDir == "" {
		logging.Agent("run: runtime.Run 拒绝（沙箱为空）", "run_id", req.RunID)
		return errors.New("沙箱目录未配置")
	}
	return nil
}

func (r *Runtime) attachRunCancel(ctx context.Context, runID, sandboxDir string, extraSandbox []string) (context.Context, func()) {
	runCtx, cancel := context.WithCancel(ctx)
	runCtx = tools.WithSandbox(runCtx, sandboxDir)
	if len(extraSandbox) > 0 {
		runCtx = tools.WithExtraSandboxRoots(runCtx, extraSandbox)
	}
	r.mu.Lock()
	r.runCancels[runID] = cancel
	r.mu.Unlock()
	return runCtx, func() {
		r.mu.Lock()
		delete(r.runCancels, runID)
		r.mu.Unlock()
		cancel()
	}
}

func (r *Runtime) buildStreamingSink(base stream.Sink, runID string) (stream.Sink, func()) {
	coalescedText := stream.NewCoalesceSink(base, runID, 100*time.Millisecond)
	coalesced := stream.NewOutputCoalesceSink(coalescedText, runID, 200*time.Millisecond)
	return coalesced, func() {
		coalesced.Close()
		coalescedText.Close()
	}
}

func (r *Runtime) runTAORSession(ctx context.Context, req ports.RunRequest, runID string, sink stream.Sink) (ports.RunResult, error) {
	mcpMgr := r.newMCPManager(req.MCP)
	registry := agent.NewRegistry()
	coordAsync := coordinator.NewAsyncSupport()
	workerRun := coordinator.NewRunControl()
	subagentsDir := filepath.Join(filepath.Dir(req.SessionsDir), "subagents")
	sidechain := agent.NewSidechainWriter(subagentsDir)
	auditWriter := audit.NewWriter(req.SessionsDir, req.SandboxDir, runID)
	hub := coordinator.NewStreamHub(runID, registry, sidechain, sink, nil,
		req.OnSubagentUpdate,
		req.OnSubagentDone,
	)
	hub.Audit = auditWriter
	client := llm.NewClient(req.Model.BaseURL, req.Model.APIKey)
	cfg, err := r.buildQueryConfig(client, req, mcpMgr, registry, coordAsync, workerRun, hub, auditWriter, runID)
	if err != nil {
		return ports.RunResult{}, err
	}
	workerRun.SetParent(ctx)
	defer workerRun.SetParent(context.Background())
	start := time.Now()
	result := query.RunSession(ctx, cfg, sink)
	_ = auditWriter.Close(audit.SessionMeta{
		StopReason: string(result.StopReason),
		TurnCount:  result.TurnCount,
		DurationMs: time.Since(start).Milliseconds(),
	})
	out := ports.RunResult{
		StopReason: string(result.StopReason),
		TurnCount:  result.TurnCount,
		Err:        result.Err,
		Messages:   result.Messages,
	}
	if result.Err != nil {
		return out, result.Err
	}
	if len(result.Messages) > 0 {
		out.Output = result.Messages[len(result.Messages)-1].Content
	}
	logging.Agent("run: runtime.Run 结束",
		"run_id", runID,
		"stop_reason", out.StopReason,
		"turn_count", out.TurnCount,
		"output_len", len(out.Output),
		"duration_ms", time.Since(start).Milliseconds(),
		"has_error", result.Err != nil,
	)
	return out, nil
}

// newMCPManager 根据调用方已组装的 MCP 配置创建管理器（不再读取 runtimeCfg.MCP）。
func (r *Runtime) newMCPManager(servers []ports.MCPServerConfig) *mcp.Manager {
	m := mcp.NewManager()
	cfgs := map[string]mcp.ServerConfig{}
	for _, e := range servers {
		if e.Disabled {
			delete(cfgs, e.Name)
			continue
		}
		cfgs[e.Name] = mcp.ServerConfig{
			Command: e.Command, Args: e.Args, URL: e.URL, Headers: e.Headers, Env: e.Env, Disabled: e.Disabled,
		}
	}
	m.UpdateConfigs(cfgs)
	return m
}

func contextPolicyFromRuntime(runtimeCfg *config.RuntimeConfig) query.ContextPolicy {
	ctxPolicy := query.ContextPolicy{
		AutoCompactThreshold: runtimeCfg.AI.Context.AutoCompactThreshold,
		KeepRecentMessages:   runtimeCfg.AI.Context.KeepRecentMessages,
	}
	if ctxPolicy.KeepRecentMessages < 1 {
		ctxPolicy.KeepRecentMessages = 8
	}
	return ctxPolicy
}

// buildQueryConfig 为 Run 构建 query.Config。
func (r *Runtime) buildQueryConfig(
	client *llm.Client,
	req ports.RunRequest,
	mcpMgr *mcp.Manager,
	registry *agent.Registry,
	coordAsync *coordinator.AsyncSupport,
	workerRun *coordinator.RunControl,
	hub *coordinator.StreamHub,
	auditWriter *audit.Writer,
	sessionID string,
) (query.Config, error) {
	reg := tools.DefaultRegistry()
	if !req.Policy.AllowShell {
		reg = tools.RegistryWithoutShell(nil)
	}
	_ = tools.RegisterMCPTools(reg, mcpMgr)
	workerOnly := coordinator.CloneWorkerRegistry(reg)
	ctxPolicy := contextPolicyFromRuntime(r.runtimeCfg)
	coordCfg := coordinator.Config{
		LLM:           client,
		Model:         req.Model.Model,
		AgentRegistry: registry,
		ToolRegistry:  workerOnly,
		CanUseTool:    func(string, map[string]any) bool { return true },
		MaxTurns:      200,
		MaxTokens:     req.Model.MaxTokens,
		ContextPolicy: ctxPolicy,
		Async:         coordAsync,
		RunControl:    workerRun,
		StreamHub:     hub,
		SessionID:     sessionID,
		Audit:         auditWriter,
		SandboxDir:    req.SandboxDir,
	}
	parentReg := coordinator.NewParentRegistry(coordCfg)
	asyncResults, hasPending := coordAsync.QueryConfigFields()
	prompt := coordinator.BuildParentSystemPrompt(workerOnly.Names(), mcpMgr.Names())
	logging.Agent("run: 构建 Query 配置", "session_id", sessionID, "sandbox", req.SandboxDir)
	return coordinator.QueryConfigFromCoordinator(coordCfg, coordinator.QueryConfigOverrides{
		SystemPrompt:    prompt,
		Registry:        parentReg,
		AsyncResults:    asyncResults,
		HasPendingAsync: hasPending,
		InitialMessages: append([]query.Message(nil), req.Messages...),
	}), nil
}

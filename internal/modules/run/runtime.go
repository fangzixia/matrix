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
	if req.Model.APIKey == "" {
		return ports.RunResult{}, errors.New("未配置 API Key")
	}
	if len(req.Messages) == 0 {
		return ports.RunResult{}, errors.New("消息不能为空")
	}
	if req.SandboxDir == "" {
		return ports.RunResult{}, errors.New("沙箱目录未配置")
	}
	runID := req.RunID
	if runID == "" {
		runID = uuid.NewString()
	}
	runCtx, cancel := context.WithCancel(ctx)
	runCtx = tools.WithSandbox(runCtx, req.SandboxDir)
	if len(req.ExtraSandboxDirs) > 0 {
		runCtx = tools.WithExtraSandboxRoots(runCtx, req.ExtraSandboxDirs)
	}
	r.mu.Lock()
	r.runCancels[runID] = cancel
	r.mu.Unlock()
	defer func() {
		r.mu.Lock()
		delete(r.runCancels, runID)
		r.mu.Unlock()
		cancel()
	}()
	coalesced := stream.NewCoalesceSink(sink, runID, 100*time.Millisecond)
	defer coalesced.Close()
	mcpMgr := r.newMCPManager(req.MCP)
	registry := coordinator.NewRegistry()
	coordAsync := coordinator.NewAsyncSupport()
	workerRun := coordinator.NewRunControl()
	subagentsDir := filepath.Join(filepath.Dir(req.SessionsDir), "subagents")
	sidechain := agent.NewSidechainWriter(subagentsDir)
	auditWriter := audit.NewWriter(req.SessionsDir, req.SandboxDir, runID)
	hub := coordinator.NewStreamHub(runID, registry, sidechain, coalesced, nil, nil, nil)
	hub.Audit = auditWriter
	client := llm.NewClient(req.Model.BaseURL, req.Model.APIKey)
	cfg, err := r.buildQueryConfig(client, req, mcpMgr, registry, coordAsync, workerRun, hub, auditWriter, runID)
	if err != nil {
		return ports.RunResult{}, err
	}
	workerRun.SetParent(runCtx)
	defer workerRun.SetParent(context.Background())
	start := time.Now()
	result := query.RunSession(runCtx, cfg, coalesced)
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
	for i := len(result.Messages) - 1; i >= 0; i-- {
		msg := result.Messages[i]
		if msg.Role != query.RoleAssistant {
			continue
		}
		text := msg.Content
		if text == "" {
			text = msg.Thinking
		}
		if text != "" {
			out.Output = text
			break
		}
	}
	return out, nil
}

// newMCPManager 根据配置创建 MCP 管理器。
func (r *Runtime) newMCPManager(extra []ports.MCPServerConfig) *mcp.Manager {
	m := mcp.NewManager()
	cfgs := map[string]mcp.ServerConfig{}
	for name, s := range r.runtimeCfg.MCP.Servers {
		if s.Disabled {
			continue
		}
		if !r.runtimeCfg.AI.Security.AllowCommandMCP && s.Command != "" {
			continue
		}
		cfgs[name] = mcp.ServerConfig{
			Command: s.Command, Args: s.Args, URL: s.URL, Headers: s.Headers, Env: s.Env, Disabled: s.Disabled,
		}
	}
	for _, e := range extra {
		if e.Disabled {
			delete(cfgs, e.Name)
			continue
		}
		if !r.runtimeCfg.AI.Security.AllowCommandMCP && e.Command != "" {
			continue
		}
		cfgs[e.Name] = mcp.ServerConfig{
			Command: e.Command, Args: e.Args, URL: e.URL, Headers: e.Headers, Env: e.Env, Disabled: e.Disabled,
		}
	}
	m.UpdateConfigs(cfgs)
	return m
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
	ctxPolicy := query.ContextPolicy{
		AutoCompactThreshold: r.runtimeCfg.AI.Context.AutoCompactThreshold,
		KeepRecentMessages:   r.runtimeCfg.AI.Context.KeepRecentMessages,
	}
	if ctxPolicy.KeepRecentMessages < 1 {
		ctxPolicy.KeepRecentMessages = 8
	}
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
	logging.Info("run: build query config", "session_id", sessionID, "sandbox", req.SandboxDir)
	return query.Config{
		LLM:             client,
		Model:           req.Model.Model,
		SystemPrompt:    prompt,
		Registry:        parentReg,
		MaxTurns:        200,
		MaxTokens:       req.Model.MaxTokens,
		ContextPolicy:   ctxPolicy,
		CanUseTool:      func(string, map[string]any) bool { return true },
		AsyncResults:    asyncResults,
		HasPendingAsync: hasPending,
		SessionID:       sessionID,
		Audit:           auditWriter,
		InitialMessages: append([]query.Message(nil), req.Messages...),
	}, nil
}

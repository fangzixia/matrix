package desktop

import (
	"context"
	"fmt"
	"matrix/internal/logger"
	"os"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	workeragent "matrix/internal/agent"
	"matrix/internal/audit"
	"matrix/internal/coordinator"
	"matrix/internal/llm"
	"matrix/internal/mcp"
	"matrix/internal/query"
	"matrix/internal/stream"
	"matrix/internal/tools"
)

// Bridge 提供前端可调用的 Go 方法
type Bridge struct {
	ctx              context.Context
	config           *Config
	client           *llm.Client
	subAgentRegistry *workeragent.Registry
	coordinatorAsync *coordinator.AsyncSupport
	workerRun        *coordinator.RunControl
	mcpManager       *mcp.Manager
	sessions         *sessionRunner
}

// NewBridge 创建新的 Bridge 实例
func NewBridge(cfg *Config) *Bridge {
	mcpManager := mcp.NewManager()

	// 初始化 MCP 配置
	if cfg.MCPServers != nil {
		mcpConfigs := make(map[string]mcp.ServerConfig)
		for name, serverCfg := range cfg.MCPServers {
			mcpConfigs[name] = mcp.ServerConfig{
				Command:     serverCfg.Command,
				Args:        serverCfg.Args,
				Env:         serverCfg.Env,
				URL:         serverCfg.URL,
				Headers:     serverCfg.Headers,
				Disabled:    serverCfg.Disabled,
				AutoApprove: serverCfg.AutoApprove,
			}
		}
		mcpManager.UpdateConfigs(mcpConfigs)
	}

	return &Bridge{
		config:           cfg,
		subAgentRegistry: coordinator.NewRegistry(),
		coordinatorAsync: coordinator.NewAsyncSupport(),
		workerRun:        coordinator.NewRunControl(),
		mcpManager:       mcpManager,
		sessions:         &sessionRunner{},
	}
}

// Startup Wails 启动回调
func (b *Bridge) Startup(ctx context.Context) {
	b.ctx = ctx
	b.updateClient()
	logger.Info("Matrix Desktop started")
	go b.autoConnectMCPServers()
}

func (b *Bridge) autoConnectMCPServers() {
	if b.mcpManager == nil {
		return
	}
	statuses := b.mcpManager.ReconnectAll()
	available := 0
	for _, s := range statuses {
		if s.Available {
			available++
		}
	}
	logger.Infof("MCP auto-connect: %d/%d servers available", available, len(statuses))
}

// Shutdown Wails 关闭回调
func (b *Bridge) Shutdown(_ context.Context) {
	logger.Info("Matrix Desktop shutdown")
	// 关闭所有 MCP 客户端
	if b.mcpManager != nil {
		b.mcpManager.Close()
	}
}

// updateClient 更新 LLM 客户端
func (b *Bridge) updateClient() {
	if b.config.Model.APIKey != "" {
		b.client = llm.NewClient(b.config.Model.BaseURL, b.config.Model.APIKey)
	}
}

func (b *Bridge) contextPolicy() query.ContextPolicy {
	autoTh := b.config.Context.AutoCompactThreshold
	if autoTh <= 0 {
		autoTh = b.config.Model.SmartCompressThreshold
	}
	keepRecent := b.config.Context.KeepRecentMessages
	if keepRecent < 1 {
		keepRecent = 8
	}
	return query.ContextPolicy{
		MicroCompactThreshold:     b.config.Context.MicroCompactThreshold,
		KeepRecentToolResults:     b.config.Context.KeepRecentToolResults,
		ContextLimitTokens:        b.config.Context.ContextLimitTokens,
		ContextSafetyMarginTokens: b.config.Context.ContextSafetyMarginTokens,
		MaxAsyncResultRunes:       b.config.Context.MaxToolResultRunes,
		AutoCompactThreshold:      autoTh,
		KeepRecentMessages:        keepRecent,
	}
}

func (b *Bridge) canUseTool() tools.CanUseToolFn {
	return func(name string, _ map[string]any) bool {
		if len(name) > 4 && name[:4] == "mcp_" {
			parts := strings.SplitN(name[4:], "_", 2)
			if len(parts) == 2 {
				serverName := parts[0]
				toolName := parts[1]
				if b.mcpManager.IsAutoApproved(serverName, toolName) {
					return true
				}
				logger.Warnf("MCP tool call requires approval: %s/%s", serverName, toolName)
				return true
			}
		}
		return true
	}
}

// buildWorkerRegistry 创建 Worker 可用工具集（内置 + MCP，不含 Coordinator 编排工具）。
func (b *Bridge) buildWorkerRegistry() (*tools.Registry, error) {
	reg := tools.DefaultRegistry()
	if err := tools.RegisterMCPTools(reg, b.mcpManager); err != nil {
		logger.Warnf("failed to register MCP tools: %v", err)
	}
	return reg, nil
}

// buildQueryConfig 构建 CC 对齐的 Coordinator 会话（父级 agent/send_message/task_stop，Worker 持执行类工具）。
func (b *Bridge) buildQueryConfig(userPrompt string, sessionID string, hub *coordinator.StreamHub, auditWriter *audit.Writer) (query.Config, error) {
	workerReg, err := b.buildWorkerRegistry()
	if err != nil {
		return query.Config{}, err
	}

	workerOnly := coordinator.CloneWorkerRegistry(workerReg)
	coordCfg := coordinator.Config{
		LLM:                b.client,
		Model:              b.config.Model.Model,
		AgentRegistry:      b.subAgentRegistry,
		ToolRegistry:       workerOnly,
		CanUseTool:         b.canUseTool(),
		MaxTurns:           200,
		MaxTokens:          b.config.Model.MaxTokens,
		ContextPolicy:      b.contextPolicy(),
		MaxToolResultRunes: b.config.Context.MaxToolResultRunes,
		Async:              b.coordinatorAsync,
		RunControl:         b.workerRun,
		StreamHub:          hub,
		EnableNestedAgents: true,
		SessionID:          sessionID,
		Audit:              auditWriter,
	}

	reg := coordinator.NewParentRegistry(coordCfg)
	asyncResults, hasPending := b.coordinatorAsync.QueryConfigFields()
	prompt := coordinator.BuildParentSystemPrompt(workerOnly.Names(), b.connectedMCPServerNames())
	logger.Info("buildQueryConfig",
		"session_id", sessionID,
		"parent_tools", reg.Names(),
		"worker_tools", len(workerOnly.Names()),
	)

	return query.Config{
		LLM:                b.client,
		Model:              b.config.Model.Model,
		SystemPrompt:       prompt,
		Registry:           reg,
		MaxTurns:           200,
		MaxTokens:          b.config.Model.MaxTokens,
		ContextPolicy:      b.contextPolicy(),
		MaxToolResultRunes: b.config.Context.MaxToolResultRunes,
		CanUseTool:         b.canUseTool(),
		AsyncResults:       asyncResults,
		HasPendingAsync:    hasPending,
		SessionID:          sessionID,
		Audit:              auditWriter,
		InitialMessages: []query.Message{
			{Role: query.RoleUser, Content: userPrompt},
		},
	}, nil
}

// RunResult 任务执行结果
type RunResult struct {
	Output   string `json:"output"`
	HasError bool   `json:"has_error"`
	Error    string `json:"error,omitempty"`
}

// RunTask 执行 Agent（非流式）。
func (b *Bridge) RunTask(task string) (*RunResult, error) {
	if b.client == nil {
		return nil, fmt.Errorf("请先配置 API Key")
	}
	return b.toRunResult(b.executeAgent(b.ctx, task, stream.NopSink{}))
}

// RunAgentSession 执行 Agent 并通过 agent:stream 推送过程消息。
func (b *Bridge) RunAgentSession(task string) (*RunResult, error) {
	return b.runAgentSession(task)
}

func (b *Bridge) workspaceRoot() string {
	if b.config.Workspace.Root == "" {
		return "."
	}
	return b.config.Workspace.Root
}

func (b *Bridge) executeAgent(ctx context.Context, task string, sink query.StreamSink) query.Result {
	if b.workerRun != nil {
		b.workerRun.SetParent(ctx)
		defer b.workerRun.SetParent(context.Background())
	}
	cfg, err := b.buildQueryConfig(b.formatUserMessage(task), "", nil, nil)
	if err != nil {
		return query.Result{StopReason: query.StopModelError, Err: err}
	}
	return query.RunSession(ctx, cfg, sink)
}

func (b *Bridge) toRunResult(r query.Result) (*RunResult, error) {
	if r.Err != nil {
		return &RunResult{HasError: true, Error: r.Err.Error()}, nil
	}
	return &RunResult{Output: r.Answer}, nil
}

// formatUserMessage 构建首轮 user 消息（工作区 + prompt）。
func (b *Bridge) formatUserMessage(task string) string {
	msg := strings.TrimSpace(task)
	ws := b.workspaceRoot()
	if ws == "" || ws == "." {
		return msg
	}
	if msg == "" {
		return fmt.Sprintf("工作区: %s", ws)
	}
	return fmt.Sprintf("工作区: %s\n\n%s", ws, msg)
}

func (b *Bridge) connectedMCPServerNames() []string {
	if b.mcpManager == nil {
		return nil
	}
	names := make([]string, 0)
	for name, st := range b.mcpManager.GetAllServerStatuses() {
		if st != nil && st.Available {
			names = append(names, name)
		}
	}
	return names
}

// GetSettings 读取用户配置
func (b *Bridge) GetSettings() (*Settings, error) {
	logger.Info("GetSettings")
	return b.config.ToSettings(), nil
}

// SaveSettings 保存用户配置并热重载
func (b *Bridge) SaveSettings(s *Settings) error {
	logger.Infof("SaveSettings: model=%s", s.Model.Model)

	b.config.FromSettings(s)

	if err := SaveConfig(b.config); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	b.updateClient()

	// 更新 MCP 配置
	if s.MCPServers != nil {
		mcpConfigs := make(map[string]mcp.ServerConfig)
		for name, serverCfg := range s.MCPServers {
			mcpConfigs[name] = mcp.ServerConfig{
				Command:     serverCfg.Command,
				Args:        serverCfg.Args,
				Env:         serverCfg.Env,
				URL:         serverCfg.URL,
				Headers:     serverCfg.Headers,
				Disabled:    serverCfg.Disabled,
				AutoApprove: serverCfg.AutoApprove,
			}
		}
		b.mcpManager.UpdateConfigs(mcpConfigs)
		go b.autoConnectMCPServers()
	}

	return nil
}

// GetWorkspace 返回当前工作区和最近列表
func (b *Bridge) GetWorkspace() (map[string]interface{}, error) {
	logger.Infof("GetWorkspace: current=%s", b.config.Workspace.Root)
	return map[string]interface{}{
		"current": b.config.Workspace.Root,
		"recent":  b.config.Workspace.Recent,
	}, nil
}

// SetWorkspace 切换工作区
func (b *Bridge) SetWorkspace(path string) error {
	logger.Infof("SetWorkspace: path=%s", path)

	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("路径不存在或不是目录")
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("无效路径")
	}

	b.config.Workspace.Root = abs

	// 更新最近列表
	recent := []string{abs}
	for _, r := range b.config.Workspace.Recent {
		if r != abs && len(recent) < 10 {
			recent = append(recent, r)
		}
	}
	b.config.Workspace.Recent = recent

	if err := SaveConfig(b.config); err != nil {
		return err
	}

	return nil
}

// OpenFolderDialog 打开系统文件夹选择对话框
func (b *Bridge) OpenFolderDialog() (string, error) {
	return runtime.OpenDirectoryDialog(b.ctx, runtime.OpenDialogOptions{
		Title: "选择工作区文件夹",
	})
}

// ReadFile 读取工作区内的文件
func (b *Bridge) ReadFile(path string) (map[string]interface{}, error) {
	fullPath := b.resolvePath(path)
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}
	return map[string]interface{}{
		"content": string(content),
	}, nil
}

// SaveFile 保存文件到工作区
func (b *Bridge) SaveFile(path, content string) error {
	fullPath := b.resolvePath(path)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}
	return os.WriteFile(fullPath, []byte(content), 0644)
}

// FileInfo 文件信息
type FileInfo struct {
	Name  string `json:"name"`
	IsDir bool   `json:"isDir"`
	Size  int64  `json:"size"`
}

// ListFiles 列出目录内容
func (b *Bridge) ListFiles(path string) (map[string]interface{}, error) {
	fullPath := b.resolvePath(path)
	entries, err := os.ReadDir(fullPath)
	if err != nil {
		return nil, fmt.Errorf("read directory: %w", err)
	}

	files := make([]FileInfo, 0, len(entries))
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		files = append(files, FileInfo{
			Name:  entry.Name(),
			IsDir: entry.IsDir(),
			Size:  info.Size(),
		})
	}

	return map[string]interface{}{
		"files": files,
	}, nil
}

// RequirementInfo 需求文件信息
type RequirementInfo struct {
	ID       string `json:"id"`
	Path     string `json:"path"`
	Title    string `json:"title"`
	FullPath string `json:"fullPath"`
}

// GetRequirements 列出工作区 .spec 下的需求文件
func (b *Bridge) GetRequirements() (map[string]interface{}, error) {
	specDir := filepath.Join(b.config.Workspace.Root, ".spec")
	entries, err := os.ReadDir(specDir)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]interface{}{"requirements": []RequirementInfo{}}, nil
		}
		return nil, err
	}

	var result []RequirementInfo
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		if !strings.HasPrefix(e.Name(), "REQ-") {
			continue
		}
		id := strings.TrimSuffix(e.Name(), ".md")
		relPath := filepath.ToSlash(filepath.Join(".spec", e.Name()))
		result = append(result, RequirementInfo{
			ID:       id,
			Path:     relPath,
			Title:    id,
			FullPath: filepath.Join(specDir, e.Name()),
		})
	}

	return map[string]interface{}{"requirements": result}, nil
}

// EvaluationInfo 评测文件信息
type EvaluationInfo struct {
	ID            string `json:"id"`
	RequirementID string `json:"requirementId"`
	Path          string `json:"path"`
	FullPath      string `json:"fullPath"`
	Round         int    `json:"round"`
	Score         int    `json:"score"`
}

// GetEvaluations 列出工作区 .spec 下的评测文件
func (b *Bridge) GetEvaluations() (map[string]interface{}, error) {
	specDir := filepath.Join(b.config.Workspace.Root, ".spec")
	entries, err := os.ReadDir(specDir)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]interface{}{"evaluations": []EvaluationInfo{}}, nil
		}
		return nil, err
	}

	var result []EvaluationInfo
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		if !strings.HasPrefix(e.Name(), "EVAL-") {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".md")
		parts := strings.SplitN(name, "-", 4)
		reqID := ""
		if len(parts) >= 3 {
			reqID = "REQ-" + parts[2]
		}
		relPath := filepath.ToSlash(filepath.Join(".spec", e.Name()))
		result = append(result, EvaluationInfo{
			ID:            name,
			RequirementID: reqID,
			Path:          relPath,
			FullPath:      filepath.Join(specDir, e.Name()),
			Round:         0,
			Score:         0,
		})
	}

	return map[string]interface{}{"evaluations": result}, nil
}

// resolvePath 解析相对路径为绝对路径
func (b *Bridge) resolvePath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(b.config.Workspace.Root, path)
}

// TestMCPServer 测试 MCP 服务器连接
func (b *Bridge) TestMCPServer(serverName string) (*MCPServerStatus, error) {
	logger.Infof("TestMCPServer: %s", serverName)

	status := b.mcpManager.TestServer(serverName)

	// 转换为前端格式
	return &MCPServerStatus{
		Name:       status.Name,
		Available:  status.Available,
		ToolCount:  status.ToolCount,
		Tools:      status.Tools,
		Error:      status.Error,
		LastTested: status.LastTested,
	}, nil
}

// TestAllMCPServers 测试所有 MCP 服务器
func (b *Bridge) TestAllMCPServers() (map[string]*MCPServerStatus, error) {
	logger.Info("TestAllMCPServers")

	statuses := b.mcpManager.ReconnectAll()

	// 转换为前端格式
	return mcpStatusesToFrontend(statuses), nil
}

func mcpStatusesToFrontend(statuses map[string]*mcp.ServerStatus) map[string]*MCPServerStatus {
	results := make(map[string]*MCPServerStatus)
	for name, status := range statuses {
		results[name] = &MCPServerStatus{
			Name:       status.Name,
			Available:  status.Available,
			ToolCount:  status.ToolCount,
			Tools:      status.Tools,
			Error:      status.Error,
			LastTested: status.LastTested,
		}
	}
	return results
}

// GetMCPServerStatus 获取 MCP 服务器状态（从缓存）
func (b *Bridge) GetMCPServerStatus(serverName string) (*MCPServerStatus, error) {
	status := b.mcpManager.GetServerStatus(serverName)

	return &MCPServerStatus{
		Name:      status.Name,
		Available: status.Available,
		ToolCount: status.ToolCount,
		Tools:     status.Tools,
		Error:     status.Error,
	}, nil
}

// GetAllMCPServerStatuses 获取所有 MCP 服务器状态
func (b *Bridge) GetAllMCPServerStatuses() (map[string]*MCPServerStatus, error) {
	logger.Info("GetAllMCPServerStatuses")

	statuses := b.mcpManager.GetAllServerStatuses()

	// 转换为前端格式
	results := make(map[string]*MCPServerStatus)
	for name, status := range statuses {
		results[name] = &MCPServerStatus{
			Name:      status.Name,
			Available: status.Available,
			ToolCount: status.ToolCount,
			Tools:     status.Tools,
			Error:     status.Error,
		}
	}

	return results, nil
}

// CallMCPTool 调用 MCP 工具
func (b *Bridge) CallMCPTool(serverName, toolName string, arguments map[string]interface{}) (map[string]interface{}, error) {
	logger.Infof("CallMCPTool: server=%s, tool=%s", serverName, toolName)

	result, err := b.mcpManager.CallTool(serverName, toolName, arguments)
	if err != nil {
		return nil, fmt.Errorf("call tool: %w", err)
	}

	// 转换为前端格式
	response := map[string]interface{}{
		"isError": result.IsError,
		"content": result.Content,
	}

	return response, nil
}

// ListMCPTools 列出 MCP 服务器的所有工具
func (b *Bridge) ListMCPTools(serverName string) ([]map[string]interface{}, error) {
	logger.Infof("ListMCPTools: server=%s", serverName)

	tools, err := b.mcpManager.ListTools(serverName)
	if err != nil {
		return nil, fmt.Errorf("list tools: %w", err)
	}

	// 转换为前端格式
	result := make([]map[string]interface{}, len(tools))
	for i, tool := range tools {
		result[i] = map[string]interface{}{
			"name":        tool.Name,
			"description": tool.Description,
			"inputSchema": tool.InputSchema,
		}
	}

	return result, nil
}

// ListAllMCPTools 列出所有 MCP 服务器的工具
func (b *Bridge) ListAllMCPTools() (map[string]interface{}, error) {
	logger.Info("ListAllMCPTools")

	allTools, err := b.mcpManager.ListAllTools()
	if err != nil {
		return nil, fmt.Errorf("list all tools: %w", err)
	}

	// 转换为前端格式
	result := make(map[string]interface{})
	for serverName, tools := range allTools {
		toolList := make([]map[string]interface{}, len(tools))
		for i, tool := range tools {
			toolList[i] = map[string]interface{}{
				"name":        tool.Name,
				"description": tool.Description,
				"inputSchema": tool.InputSchema,
			}
		}
		result[serverName] = toolList
	}

	return result, nil
}

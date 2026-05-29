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
	"matrix/internal/matrixpaths"
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
	chatTranscripts  *ChatTranscriptStore
	chatSessionStore *ChatSessionStore
	fileService      *FileService
	settingsService  *SettingsService
	workspaceService *WorkspaceService
	mcpService       *MCPService
}

// NewBridge 创建新的 Bridge 实例
func NewBridge(cfg *Config) *Bridge {
	mcpManager := mcp.NewManager()

	b := &Bridge{
		config:           cfg,
		subAgentRegistry: coordinator.NewRegistry(),
		coordinatorAsync: coordinator.NewAsyncSupport(),
		workerRun:        coordinator.NewRunControl(),
		mcpManager:       mcpManager,
		sessions:         &sessionRunner{},
	}
	b.settingsService = NewSettingsService(cfg)
	b.workspaceService = NewWorkspaceService(cfg, func(oldRoot, newRoot string) {
		if b.chatTranscripts != nil {
			b.chatTranscripts.InvalidateCache()
		}
	})
	b.mcpService = NewMCPService(mcpManager)
	b.mcpService.UpdateConfigs(mcpConfigsFromConfig(cfg))
	b.chatTranscripts = NewChatTranscriptStore(b.workspaceRoot)
	b.chatSessionStore = NewChatSessionStore(b.workspaceRoot)
	b.fileService = NewFileService(b.workspaceRoot)
	return b
}

// Startup Wails 启动回调
func (b *Bridge) Startup(ctx context.Context) {
	b.ctx = ctx
	b.updateClient()
	if root := b.workspaceRoot(); root != "" {
		if err := matrixpaths.EnsureWorkspaceStore(root); err != nil {
			logger.Warnf("workspace store init: %v", err)
		}
	}
	logger.Info("Matrix Desktop started")
	go b.autoConnectMCPServers()
}

func (b *Bridge) autoConnectMCPServers() {
	b.mcpService.AutoConnect()
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
func (b *Bridge) buildQueryConfig(initial []query.Message, sessionID string, hub *coordinator.StreamHub, auditWriter *audit.Writer) (query.Config, error) {
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
		InitialMessages:    append([]query.Message(nil), initial...),
	}, nil
}

// RunResult 任务执行结果
type RunResult struct {
	Output     string     `json:"output"`
	HasError   bool       `json:"has_error"`
	Error      string     `json:"error,omitempty"`
	ErrorCode  string     `json:"error_code,omitempty"`
	ErrorInfo  *ErrorInfo `json:"error_info,omitempty"`
	StopReason string     `json:"stop_reason,omitempty"`
	TaskKind   string     `json:"task_kind,omitempty"`
	TaskState  string     `json:"task_state,omitempty"`
}

// RunTask 执行 Agent（非流式）。
func (b *Bridge) RunTask(task string) (*RunResult, error) {
	if b.client == nil {
		return nil, errNoAPIKey()
	}
	return b.toRunResult(b.executeAgent(b.ctx, task, stream.NopSink{}))
}

// RunAgentSession 执行 Agent 并通过 agent:stream 推送过程消息（单轮任务，无历史）。
func (b *Bridge) RunAgentSession(task string) (*RunResult, error) {
	return b.runAgentSession([]query.Message{
		{Role: query.RoleUser, Content: b.formatUserMessage(task)},
	}, "", nil)
}

func (b *Bridge) workspaceRoot() string {
	if b.workspaceService != nil {
		return b.workspaceService.Root()
	}
	return matrixpaths.NormalizeWorkspacePath(b.config.Workspace.Root)
}

func (b *Bridge) executeAgent(ctx context.Context, task string, sink query.StreamSink) query.Result {
	tools.SetWorkspaceRoot(b.workspaceRoot())
	if b.workerRun != nil {
		b.workerRun.SetParent(ctx)
		defer b.workerRun.SetParent(context.Background())
	}
	cfg, err := b.buildQueryConfig([]query.Message{
		{Role: query.RoleUser, Content: b.formatUserMessage(task)},
	}, "", nil, nil)
	if err != nil {
		return query.Result{StopReason: query.StopModelError, Err: err}
	}
	return query.RunSession(ctx, cfg, sink)
}

func (b *Bridge) toRunResult(r query.Result) (*RunResult, error) {
	info := runErrorInfo(r)
	if info != nil {
		return &RunResult{
			HasError:   true,
			Error:      info.Message,
			ErrorCode:  info.Code,
			ErrorInfo:  info,
			StopReason: string(r.StopReason),
		}, nil
	}
	return &RunResult{Output: r.Answer, StopReason: string(r.StopReason)}, nil
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
	return b.settingsService.Get(), nil
}

// SaveSettings 保存用户配置并热重载
func (b *Bridge) SaveSettings(s *Settings) error {
	logger.Infof("SaveSettings: model=%s", s.Model.Model)

	mcpConfigs, err := b.settingsService.Save(s)
	if err != nil {
		return err
	}

	b.updateClient()

	if mcpConfigs != nil {
		b.mcpService.UpdateConfigs(mcpConfigs)
		go b.autoConnectMCPServers()
	}

	return nil
}

// GetWorkspace 返回当前工作区和最近列表
func (b *Bridge) GetWorkspace() (map[string]interface{}, error) {
	root := b.workspaceRoot()
	logger.Infof("GetWorkspace: current=%s", root)
	return b.workspaceService.Get(), nil
}

// SetWorkspace 切换工作区
func (b *Bridge) SetWorkspace(path string) error {
	logger.Infof("SetWorkspace: path=%s", path)
	return b.workspaceService.Set(path)
}

// OpenFolderDialog 打开系统文件夹选择对话框
func (b *Bridge) OpenFolderDialog() (string, error) {
	return runtime.OpenDirectoryDialog(b.ctx, runtime.OpenDialogOptions{
		Title: "选择工作区文件夹",
	})
}

// ReadFile 读取工作区内的文件
func (b *Bridge) ReadFile(path string) (map[string]interface{}, error) {
	content, err := b.fileService.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"content": string(content),
	}, nil
}

// SaveFile 保存文件到工作区
func (b *Bridge) SaveFile(path, content string) error {
	return b.fileService.SaveFile(path, content)
}

// FileInfo 文件信息
type FileInfo struct {
	Name  string `json:"name"`
	IsDir bool   `json:"isDir"`
	Size  int64  `json:"size"`
}

// ListFiles 列出目录内容
func (b *Bridge) ListFiles(path string) (map[string]interface{}, error) {
	files, err := b.fileService.ListFiles(path)
	if err != nil {
		return nil, err
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

// GetRequirements 列出工作区 .matrix 下的需求文件
func (b *Bridge) GetRequirements() (map[string]interface{}, error) {
	root := b.workspaceRoot()
	if root == "" || root == "." {
		return map[string]interface{}{"requirements": []RequirementInfo{}}, nil
	}
	specDir := filepath.Join(root, ".matrix")
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
		name := e.Name()
		if !strings.HasPrefix(name, "SPEC-") && !strings.HasPrefix(name, "SPEC-") {
			continue
		}
		id := strings.TrimSuffix(name, ".md")
		relPath := filepath.ToSlash(filepath.Join(".matrix", name))
		result = append(result, RequirementInfo{
			ID:       id,
			Path:     relPath,
			Title:    id,
			FullPath: filepath.Join(specDir, name),
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

// GetEvaluations 列出工作区 .matrix 下的评测文件
func (b *Bridge) GetEvaluations() (map[string]interface{}, error) {
	root := b.workspaceRoot()
	if root == "" || root == "." {
		return map[string]interface{}{"evaluations": []EvaluationInfo{}}, nil
	}
	specDir := filepath.Join(root, ".matrix")
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
			reqID = "SPEC-" + parts[2]
		}
		relPath := filepath.ToSlash(filepath.Join(".matrix", e.Name()))
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

// TestMCPServer 测试 MCP 服务器连接
func (b *Bridge) TestMCPServer(serverName string) (*MCPServerStatus, error) {
	logger.Infof("TestMCPServer: %s", serverName)
	return b.mcpService.TestServer(serverName), nil
}

// TestAllMCPServers 测试所有 MCP 服务器
func (b *Bridge) TestAllMCPServers() (map[string]*MCPServerStatus, error) {
	logger.Info("TestAllMCPServers")
	return b.mcpService.TestAllServers(), nil
}

// GetMCPServerStatus 获取 MCP 服务器状态（从缓存）
func (b *Bridge) GetMCPServerStatus(serverName string) (*MCPServerStatus, error) {
	return b.mcpService.GetServerStatus(serverName), nil
}

// GetAllMCPServerStatuses 获取所有 MCP 服务器状态
func (b *Bridge) GetAllMCPServerStatuses() (map[string]*MCPServerStatus, error) {
	logger.Info("GetAllMCPServerStatuses")
	return b.mcpService.GetAllServerStatuses(), nil
}

// CallMCPTool 调用 MCP 工具
func (b *Bridge) CallMCPTool(serverName, toolName string, arguments map[string]interface{}) (map[string]interface{}, error) {
	logger.Infof("CallMCPTool: server=%s, tool=%s", serverName, toolName)
	return b.mcpService.CallTool(serverName, toolName, arguments)
}

// ListMCPTools 列出 MCP 服务器的所有工具
func (b *Bridge) ListMCPTools(serverName string) ([]map[string]interface{}, error) {
	logger.Infof("ListMCPTools: server=%s", serverName)
	return b.mcpService.ListTools(serverName)
}

// ListAllMCPTools 列出所有 MCP 服务器的工具
func (b *Bridge) ListAllMCPTools() (map[string]interface{}, error) {
	logger.Info("ListAllMCPTools")
	return b.mcpService.ListAllTools()
}

package desktop

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"matrix/internal/agents"
	"matrix/internal/llm"
	"matrix/internal/mcp"
	"matrix/internal/query"
	"matrix/internal/tools"
)

// Bridge 提供前端可调用的 Go 方法
type Bridge struct {
	ctx           context.Context
	config        *Config
	client        *llm.Client
	agentRegistry *agents.Registry
	mcpManager    *mcp.Manager
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
		config:        cfg,
		agentRegistry: agents.NewRegistry(),
		mcpManager:    mcpManager,
	}
}

// Startup Wails 启动回调
func (b *Bridge) Startup(ctx context.Context) {
	b.ctx = ctx
	b.updateClient()
	log.Println("Matrix Desktop started")
}

// Shutdown Wails 关闭回调
func (b *Bridge) Shutdown(_ context.Context) {
	log.Println("Matrix Desktop shutdown")
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

// buildQueryConfig 构建查询配置
func (b *Bridge) buildQueryConfig(agentName, sysPrompt, userPrompt string) (query.Config, error) {
	// 创建工具注册表
	reg := tools.DefaultRegistry()

	// 注册 MCP 工具
	if err := tools.RegisterMCPTools(reg, b.mcpManager); err != nil {
		log.Printf("Warning: failed to register MCP tools: %v", err)
	}

	// 使用统一的全局配置，MaxTurns 由核心模块控制
	return query.Config{
		LLM:          b.client,
		Model:        b.config.Model.Model,
		SystemPrompt: sysPrompt,
		Registry:     reg,
		MaxTurns:     200, // 统一的最大轮次限制
		MaxTokens:    b.config.Model.MaxTokens,
		ContextPolicy: query.ContextPolicy{
			MicroCompactThreshold:     b.config.Context.MicroCompactThreshold,
			KeepRecentToolResults:     b.config.Context.KeepRecentToolResults,
			ContextLimitTokens:        b.config.Context.ContextLimitTokens,
			ContextSafetyMarginTokens: b.config.Context.ContextSafetyMarginTokens,
			MaxAsyncResultRunes:       b.config.Context.MaxToolResultRunes,
		},
		MaxToolResultRunes: b.config.Context.MaxToolResultRunes,
		CanUseTool: func(name string, args map[string]any) bool {
			// 检查是否是 MCP 工具
			if len(name) > 4 && name[:4] == "mcp_" {
				// 解析服务器名称和工具名称
				// 格式: mcp_<serverName>_<toolName>
				parts := strings.SplitN(name[4:], "_", 2)
				if len(parts) == 2 {
					serverName := parts[0]
					toolName := parts[1]

					// 检查是否自动批准
					if b.mcpManager.IsAutoApproved(serverName, toolName) {
						return true
					}

					// TODO: 实现用户确认机制
					// 目前默认允许所有 MCP 工具调用
					log.Printf("MCP tool call requires approval: %s/%s", serverName, toolName)
					return true
				}
			}
			return true
		},
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

// EventLog 事件日志
type EventLog struct {
	Type    string `json:"type"`
	Time    string `json:"time"`
	Message string `json:"message"`
}

// RunTask 执行任务（非流式）
func (b *Bridge) RunTask(agentName, task, filePath string) (*RunResult, error) {
	log.Printf("RunTask: agent=%s, task=%s, file=%s", agentName, task, filePath)

	if b.client == nil {
		return nil, fmt.Errorf("请先配置 API Key")
	}

	// 构建系统提示
	sysPrompt := b.buildSystemPrompt(agentName)

	// 构建用户提示
	userPrompt := b.buildUserPrompt(agentName, task, filePath)

	// 构建查询配置
	cfg, err := b.buildQueryConfig(agentName, sysPrompt, userPrompt)
	if err != nil {
		return nil, err
	}

	// 执行查询
	events := make(chan query.Event, 64)
	go func() {
		for range events {
			// 非流式模式，忽略事件
		}
	}()

	result := query.Run(b.ctx, cfg, events)

	if result.Err != nil {
		return &RunResult{
			Output:   "",
			HasError: true,
			Error:    result.Err.Error(),
		}, nil
	}

	return &RunResult{
		Output:   result.Answer,
		HasError: false,
	}, nil
}

// RunTaskWithProgress 执行任务并流式返回进度
func (b *Bridge) RunTaskWithProgress(agentName, task, filePath string) (*RunResult, error) {
	log.Printf("RunTaskWithProgress: agent=%s, task=%s, file=%s", agentName, task, filePath)

	if b.client == nil {
		return nil, fmt.Errorf("请先配置 API Key")
	}

	// 构建系统提示
	sysPrompt := b.buildSystemPrompt(agentName)

	// 构建用户提示
	userPrompt := b.buildUserPrompt(agentName, task, filePath)

	// 构建查询配置
	cfg, err := b.buildQueryConfig(agentName, sysPrompt, userPrompt)
	if err != nil {
		return nil, err
	}

	// 执行查询并发送事件
	events := make(chan query.Event, 64)
	startTime := time.Now()

	log.Printf("RunTaskWithProgress: 开始执行，agent=%s", agentName)

	// 使用 WaitGroup 确保所有事件都被发送
	var wg sync.WaitGroup
	wg.Add(1)
	eventCount := 0
	go func() {
		defer wg.Done()
		for ev := range events {
			eventCount++
			eventLog := b.convertEvent(ev, startTime)
			log.Printf("发送事件 #%d: type=%s, message=%s", eventCount, eventLog.Type, eventLog.Message)
			runtime.EventsEmit(b.ctx, "task:progress", eventLog)
		}
		log.Printf("RunTaskWithProgress: 事件发送完成，共 %d 个事件", eventCount)
	}()

	result := query.Run(b.ctx, cfg, events)

	// 等待所有事件发送完成
	log.Printf("RunTaskWithProgress: 等待事件发送完成...")
	wg.Wait()
	log.Printf("RunTaskWithProgress: 执行完成")

	if result.Err != nil {
		return &RunResult{
			Output:   "",
			HasError: true,
			Error:    result.Err.Error(),
		}, nil
	}

	return &RunResult{
		Output:   result.Answer,
		HasError: false,
	}, nil
}

// convertEvent 转换查询事件为前端日志格式
func (b *Bridge) convertEvent(ev query.Event, startTime time.Time) EventLog {
	elapsed := time.Since(startTime)
	timeStr := fmt.Sprintf("%02d:%02d:%02d",
		int(elapsed.Hours()),
		int(elapsed.Minutes())%60,
		int(elapsed.Seconds())%60,
	)

	switch ev.Kind {
	case query.EventTurnStart:
		return EventLog{
			Type:    "info",
			Time:    timeStr,
			Message: fmt.Sprintf("开始第 %s 轮", ev.Delta),
		}
	case query.EventThinkingDelta:
		return EventLog{
			Type:    "thinking",
			Time:    timeStr,
			Message: ev.Delta,
		}
	case query.EventTextDelta:
		return EventLog{
			Type:    "text",
			Time:    timeStr,
			Message: ev.Delta,
		}
	case query.EventToolCall:
		return EventLog{
			Type:    "tool",
			Time:    timeStr,
			Message: fmt.Sprintf("调用工具: %s", ev.ToolName),
		}
	case query.EventToolResult:
		status := "成功"
		if ev.IsError {
			status = "失败"
		}
		return EventLog{
			Type:    "tool",
			Time:    timeStr,
			Message: fmt.Sprintf("工具 %s %s", ev.ToolName, status),
		}
	default:
		return EventLog{
			Type:    "info",
			Time:    timeStr,
			Message: ev.Delta,
		}
	}
}

// buildSystemPrompt 构建系统提示（使用 agents 包）
func (b *Bridge) buildSystemPrompt(agentName string) string {
	agent, err := b.agentRegistry.Get(agentName)
	if err != nil {
		log.Printf("Agent not found: %s, using default prompt", agentName)
		return `你是一个有用的 AI 助手，可以使用文件系统工具完成任务。
调用工具前简要说明意图。收到工具结果后解读并决定下一步。
掌握足够信息时，给出清晰简洁的最终答案。`
	}

	workspaceRoot := b.config.Workspace.Root
	if workspaceRoot == "" {
		workspaceRoot = "."
	}

	return agent.BuildSystemPrompt(workspaceRoot)
}

// buildUserPrompt 构建用户提示（使用 agents 包）
func (b *Bridge) buildUserPrompt(agentName, task, filePath string) string {
	agent, err := b.agentRegistry.Get(agentName)
	if err != nil {
		log.Printf("Agent not found: %s, using default prompt", agentName)
		workspaceRoot := b.config.Workspace.Root
		if workspaceRoot == "" {
			workspaceRoot = "."
		}
		return fmt.Sprintf("工作区: %s\n\n任务描述: %s\n", workspaceRoot, task)
	}

	workspaceRoot := b.config.Workspace.Root
	if workspaceRoot == "" {
		workspaceRoot = "."
	}

	return agent.BuildUserPrompt(workspaceRoot, task, filePath)
}

// GetSettings 读取用户配置
func (b *Bridge) GetSettings() (*Settings, error) {
	log.Println("GetSettings")
	return b.config.ToSettings(), nil
}

// SaveSettings 保存用户配置并热重载
func (b *Bridge) SaveSettings(s *Settings) error {
	log.Printf("SaveSettings: model=%s", s.Model.Model)

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
	}

	return nil
}

// GetWorkspace 返回当前工作区和最近列表
func (b *Bridge) GetWorkspace() (map[string]interface{}, error) {
	log.Printf("GetWorkspace: current=%s", b.config.Workspace.Root)
	return map[string]interface{}{
		"current": b.config.Workspace.Root,
		"recent":  b.config.Workspace.Recent,
	}, nil
}

// SetWorkspace 切换工作区
func (b *Bridge) SetWorkspace(path string) error {
	log.Printf("SetWorkspace: path=%s", path)

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

// workspaceEntry 工作区历史条目（兼容性）
type workspaceEntry struct {
	Path     string    `json:"path"`
	LastUsed time.Time `json:"lastUsed"`
}

// workspaceHistoryPath 返回工作区历史文件路径（兼容旧版）
func workspaceHistoryPath() string {
	base, err := os.UserConfigDir()
	if err != nil {
		base, _ = os.UserHomeDir()
	}
	dir := filepath.Join(base, "matrix")
	_ = os.MkdirAll(dir, 0755)
	return filepath.Join(dir, "workspaces.json")
}

// loadWorkspaceHistory 加载工作区历史（兼容旧版）
func loadWorkspaceHistory() []workspaceEntry {
	p := workspaceHistoryPath()
	if p == "" {
		return nil
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return nil
	}
	var result struct {
		Recent []workspaceEntry `json:"recent"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil
	}
	return result.Recent
}

// TestMCPServer 测试 MCP 服务器连接
func (b *Bridge) TestMCPServer(serverName string) (*MCPServerStatus, error) {
	log.Printf("TestMCPServer: %s", serverName)

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
	log.Println("TestAllMCPServers")

	statuses := b.mcpManager.TestAllServers()

	// 转换为前端格式
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

	return results, nil
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
	log.Println("GetAllMCPServerStatuses")

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
	log.Printf("CallMCPTool: server=%s, tool=%s", serverName, toolName)

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
	log.Printf("ListMCPTools: server=%s", serverName)

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
	log.Println("ListAllMCPTools")

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

// Package mcp 管理 MCP 服务器连接、工具发现与生命周期。
package mcp

import (
	"fmt"
	"matrix/internal/platform/logging"
	"os"
	"sync"
	"time"
)

// ServerConfig MCP 服务配置。
type ServerConfig struct {
	Command     string            `json:"command,omitempty"`
	Args        []string          `json:"args,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
	URL         string            `json:"url,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
	Disabled    bool              `json:"disabled"`
	AutoApprove []string          `json:"autoApprove,omitempty"`
}

// ServerStatus MCP 服务状态。
type ServerStatus struct {
	Name       string      `json:"name"`
	Available  bool        `json:"available"`
	ToolCount  int         `json:"toolCount"`
	Tools      []string    `json:"tools,omitempty"`
	Error      string      `json:"error,omitempty"`
	LastTested string      `json:"lastTested,omitempty"`
	ServerInfo *ServerInfo `json:"serverInfo,omitempty"`
}

// Manager MCP 服务管理器。
type Manager struct {
	clients map[string]*Client
	configs map[string]ServerConfig
	mu      sync.RWMutex
	connect sync.Mutex // 串行化 ReconnectAll 等重连操作
}

// NewManager 创建 MCP 管理器。
func NewManager() *Manager {
	return &Manager{
		clients: make(map[string]*Client),
		configs: make(map[string]ServerConfig),
	}
}

// UpdateConfigs 更新服务配置。
func (m *Manager) UpdateConfigs(configs map[string]ServerConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 关闭已移除或禁用的服务
	for name, client := range m.clients {
		config, exists := configs[name]
		if !exists || config.Disabled {
			logging.Infof("Closing MCP server: %s", name)
			client.Close()
			delete(m.clients, name)
		}
	}

	// 更新配置
	m.configs = configs
}

// GetClient 获取或启动 MCP 客户端。
func (m *Manager) GetClient(name string) (*Client, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 已有连接则复用
	if client, exists := m.clients[name]; exists {
		return client, nil
	}

	// 查找配置
	config, exists := m.configs[name]
	if !exists {
		return nil, fmt.Errorf("server %s not configured", name)
	}

	if config.Disabled {
		return nil, fmt.Errorf("server %s is disabled", name)
	}

	// 当前仅支持 stdio 命令型服务
	if config.Command == "" {
		return nil, fmt.Errorf("server %s: remote servers not yet supported", name)
	}

	// 合并环境变量
	env := os.Environ()
	for k, v := range config.Env {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	// 创建客户端
	client, err := NewClient(config.Command, config.Args, env)
	if err != nil {
		return nil, fmt.Errorf("create client: %w", err)
	}

	// 初始化
	if err := client.Initialize(); err != nil {
		client.Close()
		return nil, fmt.Errorf("initialize: %w", err)
	}

	m.clients[name] = client
	logging.Infof("MCP server started: %s", name)

	return client, nil
}

// TestServer 测试指定服务连通性。
func (m *Manager) TestServer(name string) *ServerStatus {
	status := &ServerStatus{
		Name:       name,
		Available:  false,
		ToolCount:  0,
		Tools:      []string{},
		LastTested: time.Now().Format("2006-01-02 15:04:05"),
	}

	m.mu.RLock()
	config, exists := m.configs[name]
	m.mu.RUnlock()

	if !exists {
		status.Error = "服务未配置"
		return status
	}

	if config.Disabled {
		status.Error = "服务已禁用"
		return status
	}

	// 获取客户端
	client, err := m.GetClient(name)
	if err != nil {
		status.Error = err.Error()
		return status
	}

	// 读取服务信息
	serverInfo := client.GetServerInfo()
	if serverInfo != nil {
		status.ServerInfo = serverInfo
	}

	// 列出工具
	tools, err := client.ListTools()
	if err != nil {
		status.Error = fmt.Sprintf("列出工具失败: %v", err)
		return status
	}

	status.Available = true
	status.ToolCount = len(tools)
	for _, tool := range tools {
		status.Tools = append(status.Tools, tool.Name)
	}

	return status
}

// ReconnectAll 关闭所有连接并重新测试各服务。
func (m *Manager) ReconnectAll() map[string]*ServerStatus {
	m.connect.Lock()
	defer m.connect.Unlock()

	m.mu.Lock()
	for name, client := range m.clients {
		client.Close()
		delete(m.clients, name)
	}
	m.mu.Unlock()
	return m.TestAllServers()
}

// TestAllServers 测试全部已配置服务。
func (m *Manager) TestAllServers() map[string]*ServerStatus {
	m.mu.RLock()
	configs := make(map[string]ServerConfig)
	for k, v := range m.configs {
		configs[k] = v
	}
	m.mu.RUnlock()

	results := make(map[string]*ServerStatus)
	for name := range configs {
		results[name] = m.TestServer(name)
	}

	return results
}

// GetServerStatus 返回服务状态（不触发连接）。
func (m *Manager) GetServerStatus(name string) *ServerStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status := &ServerStatus{
		Name:      name,
		Available: false,
	}

	config, exists := m.configs[name]
	if !exists {
		status.Error = "服务未配置"
		return status
	}

	if config.Disabled {
		status.Error = "服务已禁用"
		return status
	}

	client, exists := m.clients[name]
	if !exists {
		// 未连接时仅返回配置状态；需完整信息请调用 GetClient/TestServer
		return status
	}

	serverInfo := client.GetServerInfo()
	if serverInfo != nil {
		status.ServerInfo = serverInfo
		status.Available = true
	}

	return status
}

// GetAllServerStatuses 返回所有服务状态。
func (m *Manager) GetAllServerStatuses() map[string]*ServerStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	results := make(map[string]*ServerStatus)
	for name := range m.configs {
		results[name] = m.GetServerStatus(name)
	}

	return results
}

// CallTool 调用指定服务的工具。
func (m *Manager) CallTool(serverName, toolName string, arguments map[string]interface{}) (*CallToolResult, error) {
	client, err := m.GetClient(serverName)
	if err != nil {
		return nil, fmt.Errorf("get client: %w", err)
	}

	return client.CallTool(toolName, arguments)
}

// ListTools 列出指定服务的工具。
func (m *Manager) ListTools(serverName string) ([]Tool, error) {
	client, err := m.GetClient(serverName)
	if err != nil {
		return nil, fmt.Errorf("get client: %w", err)
	}

	return client.ListTools()
}

// ListAllTools 列出所有已启用服务的工具。
func (m *Manager) ListAllTools() (map[string][]Tool, error) {
	m.mu.RLock()
	configs := make(map[string]ServerConfig)
	for k, v := range m.configs {
		if !v.Disabled {
			configs[k] = v
		}
	}
	m.mu.RUnlock()

	results := make(map[string][]Tool)
	for name := range configs {
		tools, err := m.ListTools(name)
		if err != nil {
			logging.Infof("Failed to list tools for %s: %v", name, err)
			continue
		}
		results[name] = tools
	}

	return results, nil
}

// Close 关闭所有 MCP 连接。
func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for name, client := range m.clients {
		logging.Infof("Closing MCP server: %s", name)
		client.Close()
	}

	m.clients = make(map[string]*Client)
}

// IsAutoApproved 判断工具是否自动批准。
func (m *Manager) IsAutoApproved(serverName, toolName string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	config, exists := m.configs[serverName]
	if !exists {
		return false
	}

	for _, approved := range config.AutoApprove {
		if approved == toolName || approved == "*" {
			return true
		}
	}

	return false
}

// Names 返回已配置的 MCP 服务名称列表。
func (m *Manager) Names() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]string, 0, len(m.configs))
	for name := range m.configs {
		out = append(out, name)
	}
	return out
}

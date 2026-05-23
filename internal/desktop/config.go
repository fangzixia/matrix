package desktop

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"matrix/internal/matrixpaths"
)

// Config 桌面应用配置
type Config struct {
	// 模型配置
	Model ModelConfig `json:"model"`

	// 工作区配置
	Workspace WorkspaceConfig `json:"workspace"`

	// 上下文策略
	Context ContextConfig `json:"context"`

	// MCP 服务器配置
	MCPServers map[string]MCPServerConfig `json:"mcpServers,omitempty"`
}

// ModelConfig 模型配置
type ModelConfig struct {
	BaseURL                string `json:"baseUrl"`
	APIKey                 string `json:"apiKey"`
	Model                  string `json:"model"`
	MaxTokens              int    `json:"maxTokens"`
	SmartCompressThreshold int    `json:"smartCompressThreshold"`
}

// WorkspaceConfig 工作区配置
type WorkspaceConfig struct {
	Root   string   `json:"root"`
	Recent []string `json:"recent"`
}

// ContextConfig 上下文策略配置
type ContextConfig struct {
	MicroCompactThreshold     int `json:"microCompactThreshold"`
	KeepRecentToolResults     int `json:"keepRecentToolResults"`
	ContextLimitTokens        int `json:"contextLimitTokens"`
	ContextSafetyMarginTokens int `json:"contextSafetyMarginTokens"`
	MaxToolResultRunes        int `json:"maxToolResultRunes"`
	AutoCompactThreshold      int `json:"autoCompactThreshold"`
	KeepRecentMessages        int `json:"keepRecentMessages"`
}

// MCPServerConfig MCP 服务器配置
type MCPServerConfig struct {
	Command     string            `json:"command,omitempty"`     // 本地服务器：启动命令
	Args        []string          `json:"args,omitempty"`        // 本地服务器：命令参数
	Env         map[string]string `json:"env,omitempty"`         // 本地服务器：环境变量
	URL         string            `json:"url,omitempty"`         // 远程服务器：连接地址
	Headers     map[string]string `json:"headers,omitempty"`     // 远程服务器：HTTP 头
	Disabled    bool              `json:"disabled"`              // 是否禁用
	AutoApprove []string          `json:"autoApprove,omitempty"` // 自动批准的工具列表
}

// MCPServerStatus MCP 服务器状态（运行时信息，不保存到配置文件）
type MCPServerStatus struct {
	Name       string   `json:"name"`
	Available  bool     `json:"available"`
	ToolCount  int      `json:"toolCount"`
	Tools      []string `json:"tools,omitempty"`
	Error      string   `json:"error,omitempty"`
	LastTested string   `json:"lastTested,omitempty"`
}

// ModelSettings 模型配置（前端使用）
type ModelSettings struct {
	BaseURL                string `json:"baseUrl"`
	APIKey                 string `json:"apiKey"`
	Model                  string `json:"model"`
	MaxContextTokens       int    `json:"maxContextTokens"`
	SmartCompressThreshold int    `json:"smartCompressThreshold"`
}

// Settings 前端配置结构
type Settings struct {
	Model      ModelSettings              `json:"model"`
	MCPServers map[string]MCPServerConfig `json:"mcpServers,omitempty"`
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		Model: ModelConfig{
			BaseURL:                "https://api.deepseek.com",
			APIKey:                 "",
			Model:                  "deepseek-reasoner",
			MaxTokens:              8192,
			SmartCompressThreshold: 100000,
		},
		Workspace: WorkspaceConfig{
			Root:   "",
			Recent: []string{},
		},
		Context: ContextConfig{
			MicroCompactThreshold:     100000,
			KeepRecentToolResults:     3,
			ContextLimitTokens:        196608,
			ContextSafetyMarginTokens: 2048,
			MaxToolResultRunes:        12000,
			AutoCompactThreshold:      100000,
			KeepRecentMessages:        8,
		},
		MCPServers: map[string]MCPServerConfig{},
	}
}

// ConfigDir 返回配置目录路径（与 matrixpaths.AppDataDir 相同）。
func ConfigDir() (string, error) {
	return matrixpaths.AppDataDir()
}

// ConfigPath 返回配置文件路径
func ConfigPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

// LoadConfig 加载配置
func LoadConfig() (*Config, error) {
	path, err := ConfigPath()
	if err != nil {
		return nil, err
	}

	// 如果配置文件不存在，返回默认配置
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return DefaultConfig(), nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	return &cfg, nil
}

// SaveConfig 保存配置
func SaveConfig(cfg *Config) error {
	path, err := ConfigPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	return nil
}

// ToSettings 转换为前端 Settings 格式
func (c *Config) ToSettings() *Settings {
	s := &Settings{}
	s.Model.BaseURL = c.Model.BaseURL
	s.Model.APIKey = c.Model.APIKey
	s.Model.Model = c.Model.Model
	s.Model.MaxContextTokens = c.Context.ContextLimitTokens
	s.Model.SmartCompressThreshold = c.Model.SmartCompressThreshold
	s.MCPServers = c.MCPServers
	return s
}

// FromSettings 从前端 Settings 更新配置
func (c *Config) FromSettings(s *Settings) {
	c.Model.BaseURL = s.Model.BaseURL
	c.Model.APIKey = s.Model.APIKey
	c.Model.Model = s.Model.Model
	c.Context.ContextLimitTokens = s.Model.MaxContextTokens
	c.Model.SmartCompressThreshold = s.Model.SmartCompressThreshold
	if s.Model.SmartCompressThreshold > 0 {
		c.Context.AutoCompactThreshold = s.Model.SmartCompressThreshold
	}
	if s.MCPServers != nil {
		c.MCPServers = s.MCPServers
	}
}

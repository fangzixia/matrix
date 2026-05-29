package desktop

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"

	"matrix/internal/matrixpaths"
)

const defaultModelProfileID = "default"

// Config 桌面应用配置
type Config struct {
	Models        []ModelProfile `json:"models"`
	ActiveModelID string         `json:"activeModelId"`

	// 旧版单模型字段，仅用于加载迁移，保存时不写出
	Model ModelConfig `json:"model,omitempty"`

	Workspace WorkspaceConfig `json:"workspace"`
	Context   ContextConfig   `json:"context"`

	MCPServers map[string]MCPServerConfig `json:"mcpServers,omitempty"`
}

// ModelProfile 可切换的模型配置项
type ModelProfile struct {
	ID                     string `json:"id"`
	Name                   string `json:"name"`
	BaseURL                string `json:"baseUrl"`
	APIKey                 string `json:"apiKey"`
	Model                  string `json:"model"`
	MaxTokens              int    `json:"maxTokens,omitempty"`
	SmartCompressThreshold int    `json:"smartCompressThreshold,omitempty"`
}

// ModelConfig 运行时 LLM 参数（由活动 ModelProfile 解析）
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
	Command     string            `json:"command,omitempty"`
	Args        []string          `json:"args,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
	URL         string            `json:"url,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
	Disabled    bool              `json:"disabled"`
	AutoApprove []string          `json:"autoApprove,omitempty"`
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

// ModelProfileSettings 单个模型（前端）
type ModelProfileSettings struct {
	ID                     string `json:"id"`
	Name                   string `json:"name"`
	BaseURL                string `json:"baseUrl"`
	APIKey                 string `json:"apiKey"`
	Model                  string `json:"model"`
	MaxTokens              int    `json:"maxTokens"`
	SmartCompressThreshold int    `json:"smartCompressThreshold"`
}

// ModelSettings 旧版单模型（兼容读取）
type ModelSettings struct {
	BaseURL                string `json:"baseUrl"`
	APIKey                 string `json:"apiKey"`
	Model                  string `json:"model"`
	MaxContextTokens       int    `json:"maxContextTokens"`
	SmartCompressThreshold int    `json:"smartCompressThreshold"`
}

// Settings 前端配置结构
type Settings struct {
	Models           []ModelProfileSettings     `json:"models"`
	ActiveModelID    string                     `json:"activeModelId"`
	MaxContextTokens int                        `json:"maxContextTokens"`
	Model            ModelSettings              `json:"model,omitempty"`
	MCPServers       map[string]MCPServerConfig `json:"mcpServers,omitempty"`
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	cfg := &Config{
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
	cfg.Models = []ModelProfile{defaultModelProfile()}
	cfg.ActiveModelID = defaultModelProfileID
	return cfg
}

func defaultModelProfile() ModelProfile {
	return ModelProfile{
		ID:                     defaultModelProfileID,
		Name:                   "DeepSeek",
		BaseURL:                "https://api.deepseek.com",
		APIKey:                 "",
		Model:                  "deepseek-reasoner",
		MaxTokens:              8192,
		SmartCompressThreshold: 100000,
	}
}

// ActiveModel 返回当前选中的模型配置项
func (c *Config) ActiveModel() ModelProfile {
	if c == nil || len(c.Models) == 0 {
		return defaultModelProfile()
	}
	for _, m := range c.Models {
		if m.ID == c.ActiveModelID {
			return m
		}
	}
	return c.Models[0]
}

// ActiveModelConfig 将活动模型解析为运行时 ModelConfig
func (c *Config) ActiveModelConfig() ModelConfig {
	p := c.ActiveModel()
	maxTok := p.MaxTokens
	if maxTok <= 0 {
		maxTok = 8192
	}
	smartTh := p.SmartCompressThreshold
	if smartTh <= 0 {
		smartTh = c.Context.AutoCompactThreshold
		if smartTh <= 0 {
			smartTh = 100000
		}
	}
	return ModelConfig{
		BaseURL:                p.BaseURL,
		APIKey:                 p.APIKey,
		Model:                  p.Model,
		MaxTokens:              maxTok,
		SmartCompressThreshold: smartTh,
	}
}

// SetActiveModelID 切换活动模型，存在则返回 true
func (c *Config) SetActiveModelID(id string) bool {
	for _, m := range c.Models {
		if m.ID == id {
			c.ActiveModelID = id
			return true
		}
	}
	return false
}

func profileFromLegacy(m ModelConfig) ModelProfile {
	name := m.Model
	if name == "" {
		name = "默认模型"
	}
	return ModelProfile{
		ID:                     defaultModelProfileID,
		Name:                   name,
		BaseURL:                m.BaseURL,
		APIKey:                 m.APIKey,
		Model:                  m.Model,
		MaxTokens:              m.MaxTokens,
		SmartCompressThreshold: m.SmartCompressThreshold,
	}
}

func legacyModelPopulated(m ModelConfig) bool {
	return m.BaseURL != "" || m.APIKey != "" || m.Model != "" || m.MaxTokens > 0 || m.SmartCompressThreshold > 0
}

func normalizeConfig(cfg *Config) {
	if cfg == nil {
		return
	}
	if len(cfg.Models) == 0 {
		if legacyModelPopulated(cfg.Model) {
			cfg.Models = []ModelProfile{profileFromLegacy(cfg.Model)}
		} else {
			cfg.Models = []ModelProfile{defaultModelProfile()}
		}
	}
	for i := range cfg.Models {
		if cfg.Models[i].ID == "" {
			cfg.Models[i].ID = uuid.NewString()
		}
		if cfg.Models[i].Name == "" {
			if cfg.Models[i].Model != "" {
				cfg.Models[i].Name = cfg.Models[i].Model
			} else {
				cfg.Models[i].Name = "未命名模型"
			}
		}
	}
	if cfg.ActiveModelID == "" || !cfg.SetActiveModelID(cfg.ActiveModelID) {
		cfg.ActiveModelID = cfg.Models[0].ID
	}
	cfg.Model = ModelConfig{}
}

// ConfigDir 返回配置目录路径
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
	normalizeConfig(&cfg)
	return &cfg, nil
}

// SaveConfig 保存配置
func SaveConfig(cfg *Config) error {
	path, err := ConfigPath()
	if err != nil {
		return err
	}
	normalizeConfig(cfg)
	cfg.Model = ModelConfig{}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	return nil
}

func profileToSettings(p ModelProfile) ModelProfileSettings {
	return ModelProfileSettings{
		ID:                     p.ID,
		Name:                   p.Name,
		BaseURL:                p.BaseURL,
		APIKey:                 p.APIKey,
		Model:                  p.Model,
		MaxTokens:              p.MaxTokens,
		SmartCompressThreshold: p.SmartCompressThreshold,
	}
}

func profilesFromSettings(list []ModelProfileSettings) []ModelProfile {
	out := make([]ModelProfile, 0, len(list))
	for _, s := range list {
		id := s.ID
		if id == "" {
			id = uuid.NewString()
		}
		name := s.Name
		if name == "" {
			if s.Model != "" {
				name = s.Model
			} else {
				name = "未命名模型"
			}
		}
		out = append(out, ModelProfile{
			ID:                     id,
			Name:                   name,
			BaseURL:                s.BaseURL,
			APIKey:                 s.APIKey,
			Model:                  s.Model,
			MaxTokens:              s.MaxTokens,
			SmartCompressThreshold: s.SmartCompressThreshold,
		})
	}
	return out
}

// ToSettings 转换为前端 Settings 格式
func (c *Config) ToSettings() *Settings {
	normalizeConfig(c)
	s := &Settings{
		Models:           make([]ModelProfileSettings, 0, len(c.Models)),
		ActiveModelID:    c.ActiveModelID,
		MaxContextTokens: c.Context.ContextLimitTokens,
		MCPServers:       c.MCPServers,
	}
	for _, p := range c.Models {
		s.Models = append(s.Models, profileToSettings(p))
	}
	return s
}

// FromSettings 从前端 Settings 更新配置
func (c *Config) FromSettings(s *Settings) {
	if s == nil {
		return
	}
	if len(s.Models) > 0 {
		c.Models = profilesFromSettings(s.Models)
		if s.ActiveModelID != "" {
			c.ActiveModelID = s.ActiveModelID
		}
	} else if legacyModelPopulated(ModelConfig{
		BaseURL:                s.Model.BaseURL,
		APIKey:                 s.Model.APIKey,
		Model:                  s.Model.Model,
		SmartCompressThreshold: s.Model.SmartCompressThreshold,
	}) {
		c.Models = []ModelProfile{profileFromLegacy(ModelConfig{
			BaseURL:                s.Model.BaseURL,
			APIKey:                 s.Model.APIKey,
			Model:                  s.Model.Model,
			SmartCompressThreshold: s.Model.SmartCompressThreshold,
		})}
		c.ActiveModelID = defaultModelProfileID
	}
	normalizeConfig(c)

	if s.MaxContextTokens > 0 {
		c.Context.ContextLimitTokens = s.MaxContextTokens
	}
	active := c.ActiveModel()
	if active.SmartCompressThreshold > 0 {
		c.Context.AutoCompactThreshold = active.SmartCompressThreshold
	} else if s.Model.SmartCompressThreshold > 0 {
		c.Context.AutoCompactThreshold = s.Model.SmartCompressThreshold
	}
	if s.MCPServers != nil {
		c.MCPServers = s.MCPServers
	}
}

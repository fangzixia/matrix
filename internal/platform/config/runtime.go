package config

import (
	"errors"
	"strings"
	"time"
)

// RuntimeConfig 是进程内运行时配置，由系统配置服务从数据库加载并热更新。
type RuntimeConfig struct {
	AI  AIConfig
	MCP MCPConfig
	Git GitConfig
}

// AIConfig 是 LLM 模型、上下文与安全策略。
type AIConfig struct {
	Models   []ModelProfile
	Context  ContextConfig
	Security SecurityConfig
}

// 附件类型常量（与前端共用）。
const (
	AttachmentTypeImage    = "image"
	AttachmentTypeDocument = "document"
)

// ModelProfile 系统级可配置的模型条目（支持多条启用/默认）。
type ModelProfile struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	BaseURL         string   `json:"base_url"`
	APIKey          string   `json:"api_key"`
	Model           string   `json:"model"`
	MaxTokens       int      `json:"max_tokens"`
	Enabled         bool     `json:"enabled"`
	Default         bool     `json:"default"`
	Multimodal      bool     `json:"multimodal"`
	AttachmentTypes []string `json:"attachment_types"`
}

// ModelSpec 是单个生效模型的连接参数。
type ModelSpec struct {
	BaseURL   string
	APIKey    string
	Model     string
	MaxTokens int
}

// ContextConfig 是对话上下文压缩阈值。
type ContextConfig struct {
	AutoCompactThreshold int
	KeepRecentMessages   int
}

// SecurityConfig 是 Agent 沙箱安全策略。
type SecurityConfig struct {
	AllowShell      bool
	AllowCommandMCP bool
	ShellTimeout    time.Duration
}

// MCPConfig 是 MCP 服务器映射。
type MCPConfig struct {
	Servers map[string]MCPServerConfig
}

// MCPServerConfig 是单个 MCP 服务器的连接参数。
type MCPServerConfig struct {
	Command  string
	Args     []string
	URL      string
	Disabled bool
	Headers  map[string]string
	Env      map[string]string
}

// GitConfig 是 Git 克隆与 SSH 访问配置。
type GitConfig struct {
	SSHKeyPath   string
	CloneTimeout time.Duration
	Accesses     []GitAccess
}

// GitAccess 是按主机匹配的 SSH 密钥规则。
type GitAccess struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Host       string `json:"host"`
	SSHKeyPath string `json:"ssh_key_path"`
}

// DefaultRuntime 返回运行时配置的代码内置默认值（数据库无记录时由 Bootstrap 应用）。
func DefaultRuntime() *RuntimeConfig {
	return &RuntimeConfig{
		MCP: MCPConfig{Servers: map[string]MCPServerConfig{}},
	}
}

// ToSpec 将 ModelProfile 转换为运行时 ModelSpec。
func (p ModelProfile) ToSpec() ModelSpec {
	return ModelSpec{
		BaseURL:   p.BaseURL,
		APIKey:    p.APIKey,
		Model:     p.Model,
		MaxTokens: p.MaxTokens,
	}
}

// AllowsAttachmentType 判断模型是否允许指定类型的附件。
func (p ModelProfile) AllowsAttachmentType(t string) bool {
	if !p.Multimodal {
		return false
	}
	for _, at := range p.AttachmentTypes {
		if at == t {
			return true
		}
	}
	return false
}

// ActiveModelProfile 返回用户在系统配置中启用的默认模型完整配置。
func (a AIConfig) ActiveModelProfile() (ModelProfile, bool) {
	if len(a.Models) == 0 {
		return ModelProfile{}, false
	}
	for _, m := range a.Models {
		if m.Enabled && m.Default {
			return m, true
		}
	}
	for _, m := range a.Models {
		if m.Enabled {
			return m, true
		}
	}
	return ModelProfile{}, false
}

// ActiveModel 返回用户在系统配置中启用的默认模型；未配置时 ok 为 false。
func (a AIConfig) ActiveModel() (ModelSpec, bool) {
	p, ok := a.ActiveModelProfile()
	if !ok {
		return ModelSpec{}, false
	}
	return p.ToSpec(), true
}

// EnabledModels 返回所有已启用的模型配置。
func (a AIConfig) EnabledModels() []ModelProfile {
	var out []ModelProfile
	for _, m := range a.Models {
		if m.Enabled {
			out = append(out, m)
		}
	}
	return out
}

// ModelProfileByID 按 ID 查找已启用的模型配置。
func (a AIConfig) ModelProfileByID(id string) (ModelProfile, bool) {
	id = strings.TrimSpace(id)
	if id == "" {
		return ModelProfile{}, false
	}
	for _, m := range a.Models {
		if m.Enabled && m.ID == id {
			return m, true
		}
	}
	return ModelProfile{}, false
}

// ResolveModel 解析生效模型：优先 modelID，否则回退到默认启用模型。
func (a AIConfig) ResolveModel(modelID string) (ModelSpec, ModelProfile, error) {
	if id := strings.TrimSpace(modelID); id != "" {
		p, ok := a.ModelProfileByID(id)
		if !ok {
			return ModelSpec{}, ModelProfile{}, errors.New("未找到或未启用指定模型")
		}
		if !ModelConfigured(p.ToSpec()) {
			return ModelSpec{}, ModelProfile{}, errors.New("指定模型配置不完整")
		}
		return p.ToSpec(), p, nil
	}
	p, ok := a.ActiveModelProfile()
	if !ok {
		return ModelSpec{}, ModelProfile{}, errors.New("未配置模型：请在管理区域 → 系统配置中设置并启用默认模型")
	}
	spec := p.ToSpec()
	if !ModelConfigured(spec) {
		return ModelSpec{}, ModelProfile{}, errors.New("未配置模型：请在管理区域 → 系统配置中设置并启用默认模型")
	}
	return spec, p, nil
}

// ModelConfigured 判断模型配置是否完整，可用于 Run 启动前校验。
func ModelConfigured(m ModelSpec) bool {
	return strings.TrimSpace(m.BaseURL) != "" &&
		strings.TrimSpace(m.Model) != "" &&
		strings.TrimSpace(m.APIKey) != ""
}

// NormalizeModelProfiles 补全 ID 并保证仅有一个启用的默认模型。
func NormalizeModelProfiles(models []ModelProfile) []ModelProfile {
	if len(models) == 0 {
		return models
	}
	for i := range models {
		if models[i].MaxTokens <= 0 {
			models[i].MaxTokens = 8192
		}
	}
	enabledIdx := -1
	defaultIdx := -1
	for i, m := range models {
		if !m.Enabled {
			models[i].Default = false
			continue
		}
		if enabledIdx < 0 {
			enabledIdx = i
		}
		if m.Default {
			if defaultIdx < 0 {
				defaultIdx = i
			} else {
				models[i].Default = false
			}
		}
	}
	if defaultIdx < 0 && enabledIdx >= 0 {
		models[enabledIdx].Default = true
	}
	return models
}

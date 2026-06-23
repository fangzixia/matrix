package config

import (
	"strings"
	"time"
)

// RuntimeConfig 是进程内运行时配置，由系统配置服务从数据库加载并热更新。
type RuntimeConfig struct {
	AI       AIConfig
	MCP      MCPConfig
	Git      GitConfig
	Worker   WorkerConfig
	Pipeline PipelineConfig
	Run      RunConfig
}

// AIConfig 是 LLM 模型、上下文与安全策略。
type AIConfig struct {
	Models   []ModelProfile
	Context  ContextConfig
	Security SecurityConfig
}

// ModelProfile 系统级可配置的模型条目（支持多条启用/默认）。
type ModelProfile struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	BaseURL   string `json:"base_url"`
	APIKey    string `json:"api_key"`
	Model     string `json:"model"`
	MaxTokens int    `json:"max_tokens"`
	Enabled   bool   `json:"enabled"`
	Default   bool   `json:"default"`
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

// WorkerConfig 是嵌入式任务队列 Worker 参数。
type WorkerConfig struct {
	Enabled      bool
	PollInterval time.Duration
	MaxAttempts  int
	Concurrency  int
}

// PipelineConfig 是 Harness 默认流水线阶段。
type PipelineConfig struct {
	DefaultStages   []string
	PullBeforeStage bool
}

// RunConfig 是 Run 沙箱相关内部参数。
type RunConfig struct {
	SandboxMode      string // worktree（默认）| shared
	CleanupOnFailure bool   // 失败或取消时删除 worktree
}

// SandboxModeWorktree 为每 Run 独立 worktree 沙箱（可并行）。
const SandboxModeWorktree = "worktree"

// SandboxModeShared 为共享主仓库沙箱（旧版，项目内串行）。
const SandboxModeShared = "shared"

// DefaultRuntime 返回运行时配置的代码内置默认值（数据库无记录时由 Bootstrap 应用）。
func DefaultRuntime() *RuntimeConfig {
	return &RuntimeConfig{
		MCP: MCPConfig{Servers: map[string]MCPServerConfig{}},
		Run: RunConfig{
			SandboxMode:      SandboxModeWorktree,
			CleanupOnFailure: true,
		},
	}
}

// ActiveSandboxMode 返回当前 Run 沙箱模式（默认 worktree）。
func (r *RuntimeConfig) ActiveSandboxMode() string {
	if r == nil || strings.TrimSpace(r.Run.SandboxMode) == "" {
		return SandboxModeWorktree
	}
	return strings.TrimSpace(r.Run.SandboxMode)
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

// ActiveModel 返回用户在系统配置中启用的默认模型；未配置时 ok 为 false。
func (a AIConfig) ActiveModel() (ModelSpec, bool) {
	if len(a.Models) == 0 {
		return ModelSpec{}, false
	}
	for _, m := range a.Models {
		if m.Enabled && m.Default {
			return m.ToSpec(), true
		}
	}
	for _, m := range a.Models {
		if m.Enabled {
			return m.ToSpec(), true
		}
	}
	return ModelSpec{}, false
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

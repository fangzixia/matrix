// Package config 负责从 YAML 加载应用配置、环境变量展开与默认值填充。
// AI/MCP/Git/Worker/Pipeline 等运行参数可由 Web 系统配置覆盖并写入数据库。
package config

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config 根配置结构，对应 config/config.yml 各段。
type Config struct {
	System   SystemConfig   `yaml:"system"`
	Storage  StorageConfig  `yaml:"storage"`
	Logging  LoggingConfig  `yaml:"logging"`
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Auth     AuthConfig     `yaml:"auth"`
	AI       AIConfig       `yaml:"ai"`
	MCP      MCPConfig      `yaml:"mcp"`
	Git      GitConfig      `yaml:"git"`
	Worker   WorkerConfig   `yaml:"worker"`
	Pipeline PipelineConfig `yaml:"pipeline"`
	Run      RunConfig      `yaml:"run"`
}

// SystemConfig 是运行环境标识。
type SystemConfig struct {
	Env string `yaml:"env"`
}

// StorageConfig 是本地数据目录布局。
type StorageConfig struct {
	BaseDir       string   `yaml:"base_dir"`
	DataDir       string   `yaml:"data_dir"`
	WorkspacesDir string   `yaml:"workspaces_dir"`
	AuditDir      string   `yaml:"audit_dir"`
	ExportsDir    string   `yaml:"exports_dir"`
	AllowedRoots  []string `yaml:"allowed_roots"`
}

// LoggingConfig 是日志输出选项。
type LoggingConfig struct {
	Dir        string `yaml:"dir"`
	File       string `yaml:"file"`
	Level      string `yaml:"level"`
	Format     string `yaml:"format"`
	MaxSizeMB  int    `yaml:"max_size_mb"`
	MaxBackups int    `yaml:"max_backups"`
}

// ServerConfig 是 HTTP 服务监听参数。
type ServerConfig struct {
	Addr         string        `yaml:"addr"`
	ReadTimeout  time.Duration `yaml:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout"`
}

// DatabaseConfig 是 PostgreSQL 连接与迁移选项。
type DatabaseConfig struct {
	DSN          string `yaml:"dsn"`
	MaxOpenConns int    `yaml:"max_open_conns"`
	AutoMigrate  bool   `yaml:"auto_migrate"`
	SQLMigrate   bool   `yaml:"sql_migrate"`
}

// AuthConfig 是会话与首装管理员配置。
type AuthConfig struct {
	Session   SessionConfig   `yaml:"session"`
	Bootstrap BootstrapConfig `yaml:"bootstrap"`
}

// SessionConfig 是 Session Cookie 参数。
type SessionConfig struct {
	CookieName string        `yaml:"cookie_name"`
	TTL        time.Duration `yaml:"ttl"`
	Secure     bool          `yaml:"secure"`
}

// BootstrapConfig 是首次启动时创建的管理员账户。
type BootstrapConfig struct {
	AdminUsername string `yaml:"admin_username"`
	AdminPassword string `yaml:"admin_password"`
}

// AIConfig 是 LLM 模型、上下文与安全策略。
type AIConfig struct {
	DefaultModel ModelYAML      `yaml:"default_model"`
	Models       []ModelProfile `yaml:"-"`
	Context      ContextYAML    `yaml:"context"`
	Security     SecurityYAML   `yaml:"security"`
}

// ModelYAML 是 YAML 中的默认模型配置段。
type ModelYAML struct {
	BaseURL   string `yaml:"base_url"`
	APIKey    string `yaml:"api_key"`
	Model     string `yaml:"model"`
	MaxTokens int    `yaml:"max_tokens"`
}

// ContextYAML 是对话上下文压缩阈值。
type ContextYAML struct {
	AutoCompactThreshold int `yaml:"auto_compact_threshold"`
	KeepRecentMessages   int `yaml:"keep_recent_messages"`
}

// SecurityYAML 是 Agent 沙箱安全策略。
type SecurityYAML struct {
	AllowShell      bool          `yaml:"allow_shell"`
	AllowCommandMCP bool          `yaml:"allow_command_mcp"`
	ShellTimeout    time.Duration `yaml:"shell_timeout"`
}

// MCPConfig 是 MCP 服务器映射。
type MCPConfig struct {
	Servers map[string]MCPServerYAML `yaml:"servers"`
}

// MCPServerYAML 是单个 MCP 服务器的 YAML 定义。
type MCPServerYAML struct {
	Command  string            `yaml:"command"`
	Args     []string          `yaml:"args"`
	URL      string            `yaml:"url"`
	Disabled bool              `yaml:"disabled"`
	Headers  map[string]string `yaml:"headers"`
	Env      map[string]string `yaml:"env"`
}

// GitConfig 是 Git 克隆与 SSH 访问配置。
type GitConfig struct {
	SSHKeyPath   string            `yaml:"ssh_key_path"`
	CloneTimeout time.Duration     `yaml:"clone_timeout"`
	Accesses     []GitAccessConfig `yaml:"accesses"`
}

// GitAccessConfig 是按主机匹配的 SSH 密钥规则。
type GitAccessConfig struct {
	ID         string `json:"id" yaml:"id"`
	Name       string `json:"name" yaml:"name"`
	Host       string `json:"host" yaml:"host"`
	SSHKeyPath string `json:"ssh_key_path" yaml:"ssh_key_path"`
}

// WorkerConfig 是嵌入式任务队列 Worker 参数。
type WorkerConfig struct {
	Enabled      bool          `yaml:"enabled"`
	PollInterval time.Duration `yaml:"poll_interval"`
	MaxAttempts  int           `yaml:"max_attempts"`
	Concurrency  int           `yaml:"concurrency"`
}

// PipelineConfig 是 Harness 默认流水线阶段。
type PipelineConfig struct {
	DefaultStages   []string `yaml:"default_stages"`
	PullBeforeStage bool     `yaml:"pull_before_stage"`
}

// RunConfig 是 Run 沙箱与并发相关配置。
type RunConfig struct {
	SandboxMode      string `yaml:"sandbox_mode"`       // worktree（默认）| shared
	CleanupOnFailure bool   `yaml:"cleanup_on_failure"` // failed/cancelled 时删除 worktree
}

// SandboxModeWorktree 为每 Run 独立 worktree 沙箱（可并行）。
const SandboxModeWorktree = "worktree"

// SandboxModeShared 为共享主仓库沙箱（legacy，项目内串行）。
const SandboxModeShared = "shared"

var envPattern = regexp.MustCompile(`\$\{([^}:]+)(?::-([^}]*))?\}`)

// Load 读取 YAML 配置文件，展开 ${ENV} 占位符并与 Default 合并。
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	expanded := expandEnv(string(data))
	cfg := Default()
	if err := yaml.Unmarshal([]byte(expanded), cfg); err != nil {
		return nil, err
	}
	if strings.TrimSpace(cfg.Database.DSN) == "" {
		return nil, fmt.Errorf("database.dsn is required")
	}
	return cfg, nil
}

// Default 返回内置默认值；未在 YAML 中填写的字段会使用此处定义。
func Default() *Config {
	return &Config{
		System: SystemConfig{Env: "development"},
		Storage: StorageConfig{
			BaseDir:       "./data",
			DataDir:       "./data",
			WorkspacesDir: "workspaces",
			AuditDir:      "audit",
			ExportsDir:    "exports",
		},
		Logging: LoggingConfig{
			Dir:    "./logs",
			File:   "matrix.log",
			Level:  "info",
			Format: "json",
		},
		Server: ServerConfig{
			Addr:         ":8080",
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 0,
		},
		Database: DatabaseConfig{
			MaxOpenConns: 25,
			AutoMigrate:  true,
			SQLMigrate:   true,
		},
		Auth: AuthConfig{
			Session: SessionConfig{
				CookieName: "_matrix_session",
				TTL:        720 * time.Hour,
			},
			Bootstrap: BootstrapConfig{AdminUsername: "root"},
		},
		AI: AIConfig{
			DefaultModel: ModelYAML{
				BaseURL:   "https://api.deepseek.com",
				Model:     "deepseek-chat",
				MaxTokens: 8192,
			},
			Context: ContextYAML{
				AutoCompactThreshold: 100000,
				KeepRecentMessages:   8,
			},
			Security: SecurityYAML{
				ShellTimeout: 60 * time.Second,
			},
		},
		MCP: MCPConfig{Servers: map[string]MCPServerYAML{}},
		Git: GitConfig{CloneTimeout: 300 * time.Second},
		Worker: WorkerConfig{
			Enabled:      true,
			PollInterval: 2 * time.Second,
			MaxAttempts:  3,
			Concurrency:  2,
		},
		Pipeline: PipelineConfig{
			DefaultStages:   []string{"plan", "implement", "verify", "build"},
			PullBeforeStage: true,
		},
		Run: RunConfig{
			SandboxMode:      SandboxModeWorktree,
			CleanupOnFailure: true,
		},
	}
}

// ActiveSandboxMode 返回当前 Run 沙箱模式（默认 worktree）。
func (c *Config) ActiveSandboxMode() string {
	if c == nil || strings.TrimSpace(c.Run.SandboxMode) == "" {
		return SandboxModeWorktree
	}
	return strings.TrimSpace(c.Run.SandboxMode)
}

func expandEnv(s string) string {
	return envPattern.ReplaceAllStringFunc(s, func(m string) string {
		sub := envPattern.FindStringSubmatch(m)
		if len(sub) < 2 {
			return m
		}
		if v := os.Getenv(sub[1]); v != "" {
			return v
		}
		if len(sub) > 2 {
			return sub[2]
		}
		return ""
	})
}

// ResolvePath 将相对路径解析为基于 base 的绝对路径。
func ResolvePath(base, p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return base
	}
	if filepathIsAbs(p) {
		return cleanPath(p)
	}
	return cleanPath(joinPath(base, p))
}

func filepathIsAbs(path string) bool {
	if len(path) >= 3 && path[1] == ':' {
		return true
	}
	return strings.HasPrefix(path, "/") || strings.HasPrefix(path, "\\")
}

func joinPath(elem ...string) string {
	if len(elem) == 0 {
		return ""
	}
	out := elem[0]
	for _, e := range elem[1:] {
		if e == "" {
			continue
		}
		if strings.HasSuffix(out, "/") || strings.HasSuffix(out, "\\") {
			out += strings.TrimPrefix(strings.TrimPrefix(e, "/"), "\\")
		} else {
			out += string(os.PathSeparator) + strings.TrimPrefix(strings.TrimPrefix(e, "/"), "\\")
		}
	}
	return out
}

func cleanPath(p string) string {
	return strings.ReplaceAll(strings.TrimRight(p, "/\\"), "/", string(os.PathSeparator))
}

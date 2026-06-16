package config

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

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
}

type SystemConfig struct {
	Env string `yaml:"env"`
}

type StorageConfig struct {
	BaseDir       string   `yaml:"base_dir"`
	DataDir       string   `yaml:"data_dir"`
	WorkspacesDir string   `yaml:"workspaces_dir"`
	AuditDir      string   `yaml:"audit_dir"`
	ExportsDir    string   `yaml:"exports_dir"`
	AllowedRoots  []string `yaml:"allowed_roots"`
}

type LoggingConfig struct {
	Dir        string `yaml:"dir"`
	File       string `yaml:"file"`
	Level      string `yaml:"level"`
	Format     string `yaml:"format"`
	MaxSizeMB  int    `yaml:"max_size_mb"`
	MaxBackups int    `yaml:"max_backups"`
}

type ServerConfig struct {
	Addr         string        `yaml:"addr"`
	ReadTimeout  time.Duration `yaml:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout"`
}

type DatabaseConfig struct {
	DSN          string `yaml:"dsn"`
	MaxOpenConns int    `yaml:"max_open_conns"`
	AutoMigrate  bool   `yaml:"auto_migrate"`
	SQLMigrate   bool   `yaml:"sql_migrate"`
}

type AuthConfig struct {
	Session   SessionConfig   `yaml:"session"`
	Bootstrap BootstrapConfig `yaml:"bootstrap"`
}

type SessionConfig struct {
	CookieName string        `yaml:"cookie_name"`
	TTL        time.Duration `yaml:"ttl"`
	Secure     bool          `yaml:"secure"`
}

type BootstrapConfig struct {
	AdminUsername string `yaml:"admin_username"`
	AdminPassword string `yaml:"admin_password"`
}

type AIConfig struct {
	DefaultModel ModelYAML      `yaml:"default_model"`
	Models       []ModelProfile `yaml:"-"`
	Context      ContextYAML    `yaml:"context"`
	Security     SecurityYAML   `yaml:"security"`
}

type ModelYAML struct {
	BaseURL   string `yaml:"base_url"`
	APIKey    string `yaml:"api_key"`
	Model     string `yaml:"model"`
	MaxTokens int    `yaml:"max_tokens"`
}

type ContextYAML struct {
	AutoCompactThreshold int `yaml:"auto_compact_threshold"`
	KeepRecentMessages   int `yaml:"keep_recent_messages"`
}

type SecurityYAML struct {
	AllowShell      bool          `yaml:"allow_shell"`
	AllowCommandMCP bool          `yaml:"allow_command_mcp"`
	ShellTimeout    time.Duration `yaml:"shell_timeout"`
}

type MCPConfig struct {
	Servers map[string]MCPServerYAML `yaml:"servers"`
}

type MCPServerYAML struct {
	Command  string            `yaml:"command"`
	Args     []string          `yaml:"args"`
	URL      string            `yaml:"url"`
	Disabled bool              `yaml:"disabled"`
	Headers  map[string]string `yaml:"headers"`
	Env      map[string]string `yaml:"env"`
}

type GitConfig struct {
	SSHKeyPath   string            `yaml:"ssh_key_path"`
	CloneTimeout time.Duration     `yaml:"clone_timeout"`
	Accesses     []GitAccessConfig `yaml:"accesses"`
}

type GitAccessConfig struct {
	ID         string `json:"id" yaml:"id"`
	Name       string `json:"name" yaml:"name"`
	Host       string `json:"host" yaml:"host"`
	SSHKeyPath string `json:"ssh_key_path" yaml:"ssh_key_path"`
}

type WorkerConfig struct {
	Enabled      bool          `yaml:"enabled"`
	PollInterval time.Duration `yaml:"poll_interval"`
	MaxAttempts  int           `yaml:"max_attempts"`
	Concurrency  int           `yaml:"concurrency"`
}

type PipelineConfig struct {
	DefaultStages   []string `yaml:"default_stages"`
	PullBeforeStage bool     `yaml:"pull_before_stage"`
}

var envPattern = regexp.MustCompile(`\$\{([^}:]+)(?::-([^}]*))?\}`)

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
			DefaultStages:   []string{"spec", "implement", "verify", "build"},
			PullBeforeStage: true,
		},
	}
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

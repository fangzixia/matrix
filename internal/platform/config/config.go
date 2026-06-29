// Package config 负责从 YAML 加载应用配置与环境变量展开。
package config

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config 对应 config/config.yml，仅含文件级配置。
type Config struct {
	System   SystemConfig   `yaml:"system"`
	Storage  StorageConfig  `yaml:"storage"`
	Logging  LoggingConfig  `yaml:"logging"`
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Auth     AuthConfig     `yaml:"auth"`
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
	Dir              string `yaml:"dir"`
	Level            string `yaml:"level"`
	Format           string `yaml:"format"`
	AccessFormat     string `yaml:"access_format"`
	StructuredFormat string `yaml:"structured_format"`
	RetentionDays    int    `yaml:"retention_days"`
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

var envPattern = regexp.MustCompile(`\$\{([^}:]+)(?::-([^}]*))?}`)

// Load 读取 YAML 配置文件，展开 ${ENV} 占位符；所有字段须在 config.yml 中显式配置。
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	expanded := expandEnv(string(data))
	var cfg Config
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return nil, err
	}
	normalizeLogging(&cfg.Logging)
	if err := validate(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func normalizeLogging(cfg *LoggingConfig) {
	if cfg == nil {
		return
	}
	if cfg.RetentionDays <= 0 {
		cfg.RetentionDays = 30
	}
	if strings.TrimSpace(cfg.AccessFormat) == "" {
		cfg.AccessFormat = "combined"
	}
	if strings.TrimSpace(cfg.StructuredFormat) == "" {
		cfg.StructuredFormat = "json"
	}
}

func validate(cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}
	if strings.TrimSpace(cfg.System.Env) == "" {
		return fmt.Errorf("system.env is required")
	}
	if strings.TrimSpace(cfg.Storage.BaseDir) == "" {
		return fmt.Errorf("storage.base_dir is required")
	}
	if strings.TrimSpace(cfg.Storage.DataDir) == "" {
		return fmt.Errorf("storage.data_dir is required")
	}
	if strings.TrimSpace(cfg.Storage.WorkspacesDir) == "" {
		return fmt.Errorf("storage.workspaces_dir is required")
	}
	if strings.TrimSpace(cfg.Storage.AuditDir) == "" {
		return fmt.Errorf("storage.audit_dir is required")
	}
	if strings.TrimSpace(cfg.Storage.ExportsDir) == "" {
		return fmt.Errorf("storage.exports_dir is required")
	}
	if strings.TrimSpace(cfg.Logging.Dir) == "" {
		return fmt.Errorf("logging.dir is required")
	}
	if strings.TrimSpace(cfg.Logging.Level) == "" {
		return fmt.Errorf("logging.level is required")
	}
	if strings.TrimSpace(cfg.Logging.Format) == "" {
		return fmt.Errorf("logging.format is required")
	}
	if strings.TrimSpace(cfg.Server.Addr) == "" {
		return fmt.Errorf("server.addr is required")
	}
	if cfg.Server.ReadTimeout <= 0 {
		return fmt.Errorf("server.read_timeout is required")
	}
	if strings.TrimSpace(cfg.Database.DSN) == "" {
		return fmt.Errorf("database.dsn is required")
	}
	if cfg.Database.MaxOpenConns <= 0 {
		return fmt.Errorf("database.max_open_conns is required")
	}
	if strings.TrimSpace(cfg.Auth.Session.CookieName) == "" {
		return fmt.Errorf("auth.session.cookie_name is required")
	}
	if cfg.Auth.Session.TTL <= 0 {
		return fmt.Errorf("auth.session.ttl is required")
	}
	if strings.TrimSpace(cfg.Auth.Bootstrap.AdminUsername) == "" {
		return fmt.Errorf("auth.bootstrap.admin_username is required")
	}
	if strings.TrimSpace(cfg.Auth.Bootstrap.AdminPassword) == "" {
		return fmt.Errorf("auth.bootstrap.admin_password is required")
	}
	return nil
}

// expandEnv 展开配置字符串中的 ${ENV} 环境变量占位符。
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

// filepathIsAbs 跨平台判断路径是否为绝对路径。
func filepathIsAbs(path string) bool {
	if len(path) >= 3 && path[1] == ':' {
		return true
	}
	return strings.HasPrefix(path, "/") || strings.HasPrefix(path, "\\")
}

// joinPath 跨平台拼接路径片段。
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

// cleanPath 跨平台规范化路径。
func cleanPath(p string) string {
	return strings.ReplaceAll(strings.TrimRight(p, "/\\"), "/", string(os.PathSeparator))
}

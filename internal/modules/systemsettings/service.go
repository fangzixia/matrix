package systemsettings

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"matrix/internal/platform/config"
	"matrix/internal/platform/db/models"
	"matrix/internal/platform/gitutil"
)

const rowID = "default"

// Settings 系统级可编辑配置（持久化到 PG，覆盖 YAML 默认值）。
type Settings struct {
	Models     []ModelProfileSettings       `json:"models"`
	Model      ModelSettings                `json:"model,omitempty"` // 旧版单模型，加载后迁移
	Context    ContextSettings              `json:"context"`
	Security   SecuritySettings             `json:"security"`
	MCPServers map[string]MCPServerSettings `json:"mcp_servers"`
	Git        GitSettings                  `json:"git"`
	Worker     WorkerSettings               `json:"worker"`
	Pipeline   PipelineSettings             `json:"pipeline"`
}

type ModelProfileSettings struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	BaseURL   string `json:"base_url"`
	APIKey    string `json:"api_key,omitempty"`
	APIKeySet bool   `json:"api_key_set,omitempty"`
	Model     string `json:"model"`
	MaxTokens int    `json:"max_tokens"`
	Enabled   bool   `json:"enabled"`
	Default   bool   `json:"default"`
}

// ModelSettings 旧版单模型字段（兼容数据库历史数据）。
type ModelSettings struct {
	BaseURL   string `json:"base_url"`
	APIKey    string `json:"api_key,omitempty"`
	APIKeySet bool   `json:"api_key_set"`
	Model     string `json:"model"`
	MaxTokens int    `json:"max_tokens"`
}

type ContextSettings struct {
	AutoCompactThreshold int `json:"auto_compact_threshold"`
	KeepRecentMessages   int `json:"keep_recent_messages"`
}

type SecuritySettings struct {
	AllowShell      bool   `json:"allow_shell"`
	AllowCommandMCP bool   `json:"allow_command_mcp"`
	ShellTimeout    string `json:"shell_timeout"`
}

type MCPServerSettings struct {
	Command  string            `json:"command,omitempty"`
	Args     []string          `json:"args,omitempty"`
	URL      string            `json:"url,omitempty"`
	Disabled bool              `json:"disabled,omitempty"`
	Headers  map[string]string `json:"headers,omitempty"`
	Env      map[string]string `json:"env,omitempty"`
}

type GitAccess struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Host       string `json:"host"`
	SSHKeyPath string `json:"ssh_key_path"`
}

type GitSettings struct {
	CloneTimeout      string      `json:"clone_timeout"`
	SSHKeyPath        string      `json:"ssh_key_path,omitempty"`
	Accesses          []GitAccess `json:"accesses"`
	Platform          string      `json:"platform,omitempty"`
	PlatformLabel     string      `json:"platform_label,omitempty"`
	DefaultSSHKeyPath string      `json:"default_ssh_key_path,omitempty"`
}

type WorkerSettings struct {
	Enabled      bool   `json:"enabled"`
	PollInterval string `json:"poll_interval"`
	MaxAttempts  int    `json:"max_attempts"`
	Concurrency  int    `json:"concurrency"`
}

type PipelineSettings struct {
	DefaultStages   []string `json:"default_stages"`
	PullBeforeStage bool     `json:"pull_before_stage"`
}

type Hooks struct {
	OnGitUpdate      func(config.GitConfig)
	OnPipelineUpdate func(config.PipelineConfig)
}

type Service struct {
	db    *gorm.DB
	cfg   *config.Config
	hooks Hooks
}

func NewService(db *gorm.DB, cfg *config.Config) *Service {
	return &Service{db: db, cfg: cfg}
}

func (s *Service) SetHooks(h Hooks) {
	s.hooks = h
}

// Bootstrap 启动时从数据库加载覆盖并应用到内存配置。
func (s *Service) Bootstrap(ctx context.Context) error {
	stored, err := s.loadStored(ctx)
	if err != nil {
		return err
	}
	if stored == nil {
		return nil
	}
	s.apply(*stored, true)
	return nil
}

func (s *Service) Get(ctx context.Context) (*Settings, error) {
	stored, err := s.loadStored(ctx)
	if err != nil {
		return nil, err
	}
	if stored != nil {
		s.decorateForGet(stored)
		return stored, nil
	}
	return s.fromConfig(s.cfg), nil
}

func (s *Service) Save(ctx context.Context, in Settings) (*Settings, error) {
	merged, err := s.mergeInput(ctx, in)
	if err != nil {
		return nil, err
	}
	if err := validate(merged); err != nil {
		return nil, err
	}
	return s.persist(ctx, merged)
}

// SaveMCP 仅更新 MCP 服务配置，避免部分保存时其它字段校验失败。
func (s *Service) SaveMCP(ctx context.Context, servers map[string]MCPServerSettings) (*Settings, error) {
	if servers == nil {
		servers = map[string]MCPServerSettings{}
	}
	stored, err := s.loadStored(ctx)
	if err != nil {
		return nil, err
	}
	base := s.fromConfig(s.cfg)
	if stored != nil {
		migrateLegacyModel(stored)
		base.Models = stored.Models
		base.MCPServers = stored.MCPServers
		base.Context = stored.Context
		base.Security = stored.Security
		base.Git = stored.Git
		base.Worker = stored.Worker
		base.Pipeline = stored.Pipeline
	}
	base.MCPServers = servers
	base.Model = ModelSettings{}
	return s.persist(ctx, *base)
}

func (s *Service) persist(ctx context.Context, merged Settings) (*Settings, error) {
	migrateLegacyModel(&merged)
	normalizeModelSettings(&merged)
	normalizeGitSettings(&merged.Git)
	stripGitHints(&merged.Git)
	merged.Model = ModelSettings{}
	raw, err := json.Marshal(merged)
	if err != nil {
		return nil, err
	}
	row := models.SystemSetting{
		ID:        rowID,
		Settings:  string(raw),
		UpdatedAt: time.Now(),
	}
	if err := s.db.WithContext(ctx).Save(&row).Error; err != nil {
		return nil, err
	}
	s.apply(merged, false)
	out, err := s.Get(ctx)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Service) TestGit(ctx context.Context, gitURL string) (string, error) {
	timeout := s.cfg.Git.CloneTimeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return gitutil.TestConnection(ctx, s.cfg.Git, gitURL, timeout)
}

func (s *Service) decorateForGet(st *Settings) {
	migrateLegacyModel(st)
	maskModelProfiles(st)
	normalizeGitSettings(&st.Git)
	enrichGitHints(&st.Git)
	if st.MCPServers == nil {
		st.MCPServers = map[string]MCPServerSettings{}
	}
	st.Model = ModelSettings{}
}

func (s *Service) mergeInput(ctx context.Context, in Settings) (Settings, error) {
	existing, err := s.loadStored(ctx)
	if err != nil {
		return Settings{}, err
	}
	out := in
	if out.MCPServers == nil {
		if existing != nil && existing.MCPServers != nil {
			out.MCPServers = existing.MCPServers
		} else {
			out.MCPServers = map[string]MCPServerSettings{}
		}
	}
	if existing != nil {
		mergeModelAPIKeys(&out.Models, existing.Models)
		if len(out.Models) == 0 && len(existing.Models) > 0 {
			out.Models = existing.Models
		}
		if out.Worker.MaxAttempts < 1 {
			out.Worker.MaxAttempts = existing.Worker.MaxAttempts
		}
		if out.Worker.Concurrency < 1 {
			out.Worker.Concurrency = existing.Worker.Concurrency
		}
		if strings.TrimSpace(out.Worker.PollInterval) == "" {
			out.Worker.PollInterval = existing.Worker.PollInterval
		}
	} else {
		if out.Worker.MaxAttempts < 1 {
			out.Worker.MaxAttempts = s.cfg.Worker.MaxAttempts
		}
		if out.Worker.Concurrency < 1 {
			out.Worker.Concurrency = s.cfg.Worker.Concurrency
		}
		if strings.TrimSpace(out.Worker.PollInterval) == "" {
			out.Worker.PollInterval = s.cfg.Worker.PollInterval.String()
		}
	}
	normalizeModelSettings(&out)
	out.Model = ModelSettings{}
	normalizeGitSettings(&out.Git)
	stripGitHints(&out.Git)
	for i := range out.Git.Accesses {
		if out.Git.Accesses[i].ID == "" {
			out.Git.Accesses[i].ID = uuid.NewString()
		}
	}
	return out, nil
}

func (s *Service) loadStored(ctx context.Context) (*Settings, error) {
	var row models.SystemSetting
	err := s.db.WithContext(ctx).Where("id = ?", rowID).First(&row).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if row.Settings == "" || row.Settings == "{}" {
		return nil, nil
	}
	var st Settings
	if err := json.Unmarshal([]byte(row.Settings), &st); err != nil {
		return nil, fmt.Errorf("system settings: parse: %w", err)
	}
	migrateLegacyModel(&st)
	return &st, nil
}

func (s *Service) apply(st Settings, _ bool) {
	migrateLegacyModel(&st)
	normalizeModelSettings(&st)
	s.cfg.AI.Models = toConfigModels(st.Models)
	config.SyncDefaultModel(&s.cfg.AI)
	if st.Context.AutoCompactThreshold > 0 {
		s.cfg.AI.Context.AutoCompactThreshold = st.Context.AutoCompactThreshold
	}
	if st.Context.KeepRecentMessages > 0 {
		s.cfg.AI.Context.KeepRecentMessages = st.Context.KeepRecentMessages
	}
	s.cfg.AI.Security.AllowShell = st.Security.AllowShell
	s.cfg.AI.Security.AllowCommandMCP = st.Security.AllowCommandMCP
	if d, err := time.ParseDuration(st.Security.ShellTimeout); err == nil && d > 0 {
		s.cfg.AI.Security.ShellTimeout = d
	}
	if st.MCPServers != nil {
		s.cfg.MCP.Servers = toMCPServers(st.MCPServers)
	} else {
		s.cfg.MCP.Servers = map[string]config.MCPServerYAML{}
	}
	s.cfg.Git = toGitConfig(st.Git)
	if d, err := time.ParseDuration(st.Git.CloneTimeout); err == nil && d > 0 {
		s.cfg.Git.CloneTimeout = d
	}
	s.cfg.Worker.Enabled = st.Worker.Enabled
	if d, err := time.ParseDuration(st.Worker.PollInterval); err == nil && d > 0 {
		s.cfg.Worker.PollInterval = d
	}
	if st.Worker.MaxAttempts > 0 {
		s.cfg.Worker.MaxAttempts = st.Worker.MaxAttempts
	}
	if st.Worker.Concurrency > 0 {
		s.cfg.Worker.Concurrency = st.Worker.Concurrency
	}
	if len(st.Pipeline.DefaultStages) > 0 {
		s.cfg.Pipeline.DefaultStages = append([]string(nil), st.Pipeline.DefaultStages...)
	}
	s.cfg.Pipeline.PullBeforeStage = st.Pipeline.PullBeforeStage

	if s.hooks.OnGitUpdate != nil {
		s.hooks.OnGitUpdate(s.cfg.Git)
	}
	if s.hooks.OnPipelineUpdate != nil {
		s.hooks.OnPipelineUpdate(s.cfg.Pipeline)
	}
}

func (s *Service) fromConfig(cfg *config.Config) *Settings {
	git := fromGitConfig(cfg.Git)
	normalizeGitSettings(&git)
	enrichGitHints(&git)
	models := fromConfigModels(cfg.AI.Models, cfg.AI.DefaultModel)
	if len(models) == 0 {
		models = []ModelProfileSettings{defaultModelFromYAML(cfg.AI.DefaultModel)}
	}
	maskModelProfiles(&Settings{Models: models})
	return &Settings{
		Models: models,
		Context: ContextSettings{
			AutoCompactThreshold: cfg.AI.Context.AutoCompactThreshold,
			KeepRecentMessages:   cfg.AI.Context.KeepRecentMessages,
		},
		Security: SecuritySettings{
			AllowShell:      cfg.AI.Security.AllowShell,
			AllowCommandMCP: cfg.AI.Security.AllowCommandMCP,
			ShellTimeout:    cfg.AI.Security.ShellTimeout.String(),
		},
		MCPServers: fromMCPServers(cfg.MCP.Servers),
		Git:        git,
		Worker: WorkerSettings{
			Enabled:      cfg.Worker.Enabled,
			PollInterval: cfg.Worker.PollInterval.String(),
			MaxAttempts:  cfg.Worker.MaxAttempts,
			Concurrency:  cfg.Worker.Concurrency,
		},
		Pipeline: PipelineSettings{
			DefaultStages:   append([]string(nil), cfg.Pipeline.DefaultStages...),
			PullBeforeStage: cfg.Pipeline.PullBeforeStage,
		},
	}
}

func normalizeGitSettings(g *GitSettings) {
	if len(g.Accesses) == 0 && g.SSHKeyPath != "" {
		g.Accesses = []GitAccess{{
			ID: "default", Name: "默认", Host: "*", SSHKeyPath: g.SSHKeyPath,
		}}
	}
}

func toGitConfig(g GitSettings) config.GitConfig {
	normalizeGitSettings(&g)
	out := config.GitConfig{SSHKeyPath: g.SSHKeyPath}
	if d, err := time.ParseDuration(g.CloneTimeout); err == nil && d > 0 {
		out.CloneTimeout = d
	}
	for _, a := range g.Accesses {
		out.Accesses = append(out.Accesses, config.GitAccessConfig{
			ID: a.ID, Name: a.Name, Host: a.Host, SSHKeyPath: a.SSHKeyPath,
		})
	}
	if out.SSHKeyPath == "" && len(out.Accesses) > 0 {
		for _, a := range out.Accesses {
			if a.SSHKeyPath != "" {
				out.SSHKeyPath = a.SSHKeyPath
				break
			}
		}
	}
	return out
}

func fromGitConfig(g config.GitConfig) GitSettings {
	out := GitSettings{
		SSHKeyPath:   g.SSHKeyPath,
		CloneTimeout: g.CloneTimeout.String(),
	}
	for _, a := range g.Accesses {
		out.Accesses = append(out.Accesses, GitAccess{
			ID: a.ID, Name: a.Name, Host: a.Host, SSHKeyPath: a.SSHKeyPath,
		})
	}
	normalizeGitSettings(&out)
	return out
}

func enrichGitHints(g *GitSettings) {
	g.Platform = gitutil.ServerPlatform()
	g.PlatformLabel = gitutil.PlatformLabel(g.Platform)
	g.DefaultSSHKeyPath = gitutil.DefaultSSHKeyPath()
}

func stripGitHints(g *GitSettings) {
	g.Platform = ""
	g.PlatformLabel = ""
	g.DefaultSSHKeyPath = ""
}

func validate(in Settings) error {
	migrateLegacyModel(&in)
	for _, m := range in.Models {
		if m.MaxTokens < 0 {
			return fmt.Errorf("模型 %s 的 max_tokens 不能为负数", m.Name)
		}
		if m.Enabled && strings.TrimSpace(m.Model) == "" {
			return fmt.Errorf("已启用的模型 %s 须填写模型名称", m.Name)
		}
	}
	if in.Worker.MaxAttempts < 1 {
		return fmt.Errorf("max_attempts 至少为 1")
	}
	if in.Worker.Concurrency < 1 {
		return fmt.Errorf("concurrency 至少为 1")
	}
	for _, d := range []struct {
		name string
		val  string
	}{
		{"shell_timeout", in.Security.ShellTimeout},
		{"clone_timeout", in.Git.CloneTimeout},
		{"poll_interval", in.Worker.PollInterval},
	} {
		if d.val == "" {
			continue
		}
		if _, err := time.ParseDuration(d.val); err != nil {
			return fmt.Errorf("%s 格式无效，请使用如 60s、2m", d.name)
		}
	}
	return nil
}

func toMCPServers(in map[string]MCPServerSettings) map[string]config.MCPServerYAML {
	out := make(map[string]config.MCPServerYAML, len(in))
	for name, srv := range in {
		out[name] = config.MCPServerYAML{
			Command:  srv.Command,
			Args:     append([]string(nil), srv.Args...),
			URL:      srv.URL,
			Disabled: srv.Disabled,
			Headers:  srv.Headers,
			Env:      srv.Env,
		}
	}
	return out
}

func fromMCPServers(in map[string]config.MCPServerYAML) map[string]MCPServerSettings {
	if in == nil {
		return map[string]MCPServerSettings{}
	}
	out := make(map[string]MCPServerSettings, len(in))
	for name, srv := range in {
		out[name] = MCPServerSettings{
			Command:  srv.Command,
			Args:     append([]string(nil), srv.Args...),
			URL:      srv.URL,
			Disabled: srv.Disabled,
			Headers:  srv.Headers,
			Env:      srv.Env,
		}
	}
	return out
}

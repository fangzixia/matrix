// Package systemsettings 管理系统级运行参数：按业务域分表存储，启动时加载并热更新内存配置。
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

// ModelProfileSettings 单个 LLM 模型配置（含 API Key 脱敏字段）。
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

// ContextSettings 对话上下文压缩策略。
type ContextSettings struct {
	AutoCompactThreshold int `json:"auto_compact_threshold"`
	KeepRecentMessages   int `json:"keep_recent_messages"`
}

// SecuritySettings Agent 沙箱安全策略。
type SecuritySettings struct {
	AllowShell      bool   `json:"allow_shell"`
	AllowCommandMCP bool   `json:"allow_command_mcp"`
	ShellTimeout    string `json:"shell_timeout"`
}

// MCPServerSettings 单个 MCP 服务器连接参数。
type MCPServerSettings struct {
	Command  string            `json:"command,omitempty"`
	Args     []string          `json:"args,omitempty"`
	URL      string            `json:"url,omitempty"`
	Disabled bool              `json:"disabled,omitempty"`
	Headers  map[string]string `json:"headers,omitempty"`
	Env      map[string]string `json:"env,omitempty"`
}

// GitAccess 按主机匹配的 SSH 密钥访问规则。
type GitAccess struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Host       string `json:"host"`
	SSHKeyPath string `json:"ssh_key_path"`
}

// GitSettings Git 克隆超时与 SSH 访问列表。
type GitSettings struct {
	CloneTimeout      string      `json:"clone_timeout"`
	SSHKeyPath        string      `json:"ssh_key_path,omitempty"`
	Accesses          []GitAccess `json:"accesses"`
	Platform          string      `json:"platform,omitempty"`
	PlatformLabel     string      `json:"platform_label,omitempty"`
	DefaultSSHKeyPath string      `json:"default_ssh_key_path,omitempty"`
}

// WorkerSettings 嵌入式任务队列 Worker 参数。
type WorkerSettings struct {
	Enabled      bool   `json:"enabled"`
	PollInterval string `json:"poll_interval"`
	MaxAttempts  int    `json:"max_attempts"`
	Concurrency  int    `json:"concurrency"`
}

// PipelineSettings Harness 流水线默认阶段。
type PipelineSettings struct {
	DefaultStages   []string `json:"default_stages"`
	PullBeforeStage bool     `json:"pull_before_stage"`
}

// Hooks 配置变更回调，用于同步 Git/流水线到依赖服务。
type Hooks struct {
	OnGitUpdate      func(config.GitConfig)
	OnPipelineUpdate func(config.PipelineConfig)
}

// Service 系统配置读写：DB 持久化 + 内存 cfg 热更新。
type Service struct {
	db    *gorm.DB
	cfg   *config.Config
	hooks Hooks
}

// NewService 创建系统配置服务实例。
func NewService(db *gorm.DB, cfg *config.Config) *Service {
	return &Service{db: db, cfg: cfg}
}

// SetHooks 注册配置变更回调，用于同步 Git 与流水线到依赖服务。
func (s *Service) SetHooks(h Hooks) {
	s.hooks = h
}

// Bootstrap 启动时从数据库按域加载并应用到内存配置。
func (s *Service) Bootstrap(ctx context.Context) error {
	stored, err := s.loadAllStored(ctx)
	if err != nil {
		return err
	}
	if stored == nil {
		return nil
	}
	s.apply(*stored, true)
	return nil
}

// --- AI ---

// GetAI 读取 AI 系统设置。
func (s *Service) GetAI(ctx context.Context) (*AISettings, error) {
	stored, err := s.loadDomainAI(ctx)
	if err != nil {
		return nil, err
	}
	if stored != nil {
		s.decorateAIForGet(stored)
		return stored, nil
	}
	full := s.fromConfig(s.cfg)
	return &AISettings{
		Models: full.Models, Context: full.Context, Security: full.Security,
	}, nil
}

// SaveAI 保存 AI 系统设置。
func (s *Service) SaveAI(ctx context.Context, in AISettings) (*AISettings, error) {
	merged, err := s.mergeAIInput(ctx, in)
	if err != nil {
		return nil, err
	}
	if err := validateAI(merged); err != nil {
		return nil, err
	}
	if err := s.saveDomain(ctx, DomainAI, merged); err != nil {
		return nil, err
	}
	if err := s.reloadAndApply(ctx); err != nil {
		return nil, err
	}
	return s.GetAI(ctx)
}

// --- MCP ---

// GetMCP 读取 MCP 系统设置。
func (s *Service) GetMCP(ctx context.Context) (*MCPSettings, error) {
	stored, err := s.loadDomainMCP(ctx)
	if err != nil {
		return nil, err
	}
	if stored != nil {
		if stored.MCPServers == nil {
			stored.MCPServers = map[string]MCPServerSettings{}
		}
		return stored, nil
	}
	full := s.fromConfig(s.cfg)
	return &MCPSettings{MCPServers: full.MCPServers}, nil
}

// SaveMCP 保存 MCP 系统设置。
func (s *Service) SaveMCP(ctx context.Context, servers map[string]MCPServerSettings) (*MCPSettings, error) {
	if servers == nil {
		servers = map[string]MCPServerSettings{}
	}
	payload := MCPSettings{MCPServers: servers}
	if err := s.saveDomain(ctx, DomainMCP, payload); err != nil {
		return nil, err
	}
	if err := s.reloadAndApply(ctx); err != nil {
		return nil, err
	}
	return s.GetMCP(ctx)
}

// --- Git ---

// GetGit 读取 Git 系统设置。
func (s *Service) GetGit(ctx context.Context) (*GitSettings, error) {
	stored, err := s.loadDomainGit(ctx)
	if err != nil {
		return nil, err
	}
	if stored != nil {
		normalizeGitSettings(stored)
		enrichGitHints(stored)
		return stored, nil
	}
	full := s.fromConfig(s.cfg)
	return &full.Git, nil
}

// SaveGit 保存 Git 系统设置。
func (s *Service) SaveGit(ctx context.Context, in GitSettings) (*GitSettings, error) {
	merged := in
	normalizeGitSettings(&merged)
	stripGitHints(&merged)
	for i := range merged.Accesses {
		if merged.Accesses[i].ID == "" {
			merged.Accesses[i].ID = uuid.NewString()
		}
	}
	if err := validateGit(merged); err != nil {
		return nil, err
	}
	if err := s.saveDomain(ctx, DomainGit, merged); err != nil {
		return nil, err
	}
	if err := s.reloadAndApply(ctx); err != nil {
		return nil, err
	}
	return s.GetGit(ctx)
}

// TestGit 测试 Git 连通性。
func (s *Service) TestGit(ctx context.Context, gitURL string) (string, error) {
	timeout := s.cfg.Git.CloneTimeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return gitutil.TestConnection(ctx, s.cfg.Git, gitURL, timeout)
}

// --- Worker ---

// GetWorker 读取 Worker 系统设置。
func (s *Service) GetWorker(ctx context.Context) (*WorkerSettings, error) {
	stored, err := s.loadDomainWorker(ctx)
	if err != nil {
		return nil, err
	}
	if stored != nil {
		return stored, nil
	}
	full := s.fromConfig(s.cfg)
	return &full.Worker, nil
}

// SaveWorker 保存 Worker 系统设置。
func (s *Service) SaveWorker(ctx context.Context, in WorkerSettings) (*WorkerSettings, error) {
	merged, err := s.mergeWorkerInput(ctx, in)
	if err != nil {
		return nil, err
	}
	if err := validateWorker(merged); err != nil {
		return nil, err
	}
	if err := s.saveDomain(ctx, DomainWorker, merged); err != nil {
		return nil, err
	}
	if err := s.reloadAndApply(ctx); err != nil {
		return nil, err
	}
	return s.GetWorker(ctx)
}

// --- Pipeline ---

// GetPipeline 读取 Pipeline 系统设置。
func (s *Service) GetPipeline(ctx context.Context) (*PipelineSettings, error) {
	stored, err := s.loadDomainPipeline(ctx)
	if err != nil {
		return nil, err
	}
	if stored != nil {
		return stored, nil
	}
	full := s.fromConfig(s.cfg)
	return &full.Pipeline, nil
}

// SavePipeline 保存 Pipeline 系统设置。
func (s *Service) SavePipeline(ctx context.Context, in PipelineSettings) (*PipelineSettings, error) {
	if err := validatePipeline(in); err != nil {
		return nil, err
	}
	if err := s.saveDomain(ctx, DomainPipeline, in); err != nil {
		return nil, err
	}
	if err := s.reloadAndApply(ctx); err != nil {
		return nil, err
	}
	return s.GetPipeline(ctx)
}

// --- 存储层 ---

func (s *Service) saveDomain(ctx context.Context, domainID string, data any) error {
	raw, err := json.Marshal(data)
	if err != nil {
		return err
	}
	row := models.SystemSetting{
		ID:        domainID,
		Settings:  string(raw),
		UpdatedAt: time.Now(),
	}
	return s.db.WithContext(ctx).Save(&row).Error
}

func (s *Service) loadDomainRaw(ctx context.Context, domainID string) (json.RawMessage, error) {
	var row models.SystemSetting
	err := s.db.WithContext(ctx).Where("id = ?", domainID).First(&row).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if row.Settings == "" || row.Settings == "{}" {
		return nil, nil
	}
	return json.RawMessage(row.Settings), nil
}

func (s *Service) loadDomainAI(ctx context.Context) (*AISettings, error) {
	raw, err := s.loadDomainRaw(ctx, DomainAI)
	if err != nil || raw == nil {
		return nil, err
	}
	var st AISettings
	if err := json.Unmarshal(raw, &st); err != nil {
		return nil, fmt.Errorf("ai settings: parse: %w", err)
	}
	return &st, nil
}

func (s *Service) loadDomainMCP(ctx context.Context) (*MCPSettings, error) {
	raw, err := s.loadDomainRaw(ctx, DomainMCP)
	if err != nil || raw == nil {
		return nil, err
	}
	var st MCPSettings
	if err := json.Unmarshal(raw, &st); err != nil {
		return nil, fmt.Errorf("mcp settings: parse: %w", err)
	}
	return &st, nil
}

func (s *Service) loadDomainGit(ctx context.Context) (*GitSettings, error) {
	raw, err := s.loadDomainRaw(ctx, DomainGit)
	if err != nil || raw == nil {
		return nil, err
	}
	var st GitSettings
	if err := json.Unmarshal(raw, &st); err != nil {
		return nil, fmt.Errorf("git settings: parse: %w", err)
	}
	return &st, nil
}

func (s *Service) loadDomainWorker(ctx context.Context) (*WorkerSettings, error) {
	raw, err := s.loadDomainRaw(ctx, DomainWorker)
	if err != nil || raw == nil {
		return nil, err
	}
	var st WorkerSettings
	if err := json.Unmarshal(raw, &st); err != nil {
		return nil, fmt.Errorf("worker settings: parse: %w", err)
	}
	return &st, nil
}

func (s *Service) loadDomainPipeline(ctx context.Context) (*PipelineSettings, error) {
	raw, err := s.loadDomainRaw(ctx, DomainPipeline)
	if err != nil || raw == nil {
		return nil, err
	}
	var st PipelineSettings
	if err := json.Unmarshal(raw, &st); err != nil {
		return nil, fmt.Errorf("pipeline settings: parse: %w", err)
	}
	return &st, nil
}

func (s *Service) loadAllStored(ctx context.Context) (*Settings, error) {
	ai, _ := s.loadDomainAI(ctx)
	mcp, _ := s.loadDomainMCP(ctx)
	git, _ := s.loadDomainGit(ctx)
	worker, _ := s.loadDomainWorker(ctx)
	pipeline, _ := s.loadDomainPipeline(ctx)

	if ai == nil && mcp == nil && git == nil && worker == nil && pipeline == nil {
		return nil, nil
	}

	base := s.fromConfig(s.cfg)
	if ai != nil {
		base.Models = ai.Models
		base.Context = ai.Context
		base.Security = ai.Security
	}
	if mcp != nil {
		base.MCPServers = mcp.MCPServers
	}
	if git != nil {
		base.Git = *git
		stripGitHints(&base.Git)
	}
	if worker != nil {
		base.Worker = *worker
	}
	if pipeline != nil {
		base.Pipeline = *pipeline
	}
	return base, nil
}

func (s *Service) reloadAndApply(ctx context.Context) error {
	stored, err := s.loadAllStored(ctx)
	if err != nil {
		return err
	}
	if stored != nil {
		s.apply(*stored, false)
	}
	return nil
}

func (s *Service) decorateAIForGet(st *AISettings) {
	maskModelProfiles(&st.Models)
}

func (s *Service) mergeAIInput(ctx context.Context, in AISettings) (AISettings, error) {
	existing, err := s.loadDomainAI(ctx)
	if err != nil {
		return AISettings{}, err
	}
	out := in
	if existing != nil {
		mergeModelAPIKeys(&out.Models, existing.Models)
	}
	normalizeModelProfiles(&out.Models)
	return out, nil
}

func (s *Service) mergeWorkerInput(ctx context.Context, in WorkerSettings) (WorkerSettings, error) {
	existing, err := s.loadDomainWorker(ctx)
	if err != nil {
		return WorkerSettings{}, err
	}
	out := in
	if existing != nil {
		if out.MaxAttempts < 1 {
			out.MaxAttempts = existing.MaxAttempts
		}
		if out.Concurrency < 1 {
			out.Concurrency = existing.Concurrency
		}
		if strings.TrimSpace(out.PollInterval) == "" {
			out.PollInterval = existing.PollInterval
		}
	} else {
		if out.MaxAttempts < 1 {
			out.MaxAttempts = s.cfg.Worker.MaxAttempts
		}
		if out.Concurrency < 1 {
			out.Concurrency = s.cfg.Worker.Concurrency
		}
		if strings.TrimSpace(out.PollInterval) == "" {
			out.PollInterval = s.cfg.Worker.PollInterval.String()
		}
	}
	return out, nil
}

func (s *Service) apply(st Settings, _ bool) {
	normalizeModelProfiles(&st.Models)
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
	models := fromConfigModels(cfg.AI.Models)
	if len(models) == 0 {
		models = []ModelProfileSettings{defaultModelFromYAML(cfg.AI.DefaultModel)}
	}
	maskModelProfiles(&models)
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

func validateAI(in AISettings) error {
	for _, m := range in.Models {
		if m.MaxTokens < 0 {
			return fmt.Errorf("模型 %s 的 max_tokens 不能为负数", m.Name)
		}
		if m.Enabled && strings.TrimSpace(m.Model) == "" {
			return fmt.Errorf("已启用的模型 %s 须填写模型名称", m.Name)
		}
	}
	if in.Security.ShellTimeout != "" {
		if _, err := time.ParseDuration(in.Security.ShellTimeout); err != nil {
			return fmt.Errorf("shell_timeout 格式无效，请使用如 60s、2m")
		}
	}
	return nil
}

func validateGit(in GitSettings) error {
	if in.CloneTimeout != "" {
		if _, err := time.ParseDuration(in.CloneTimeout); err != nil {
			return fmt.Errorf("clone_timeout 格式无效，请使用如 300s、5m")
		}
	}
	return nil
}

func validateWorker(in WorkerSettings) error {
	if in.MaxAttempts < 1 {
		return fmt.Errorf("max_attempts 至少为 1")
	}
	if in.Concurrency < 1 {
		return fmt.Errorf("concurrency 至少为 1")
	}
	if in.PollInterval != "" {
		if _, err := time.ParseDuration(in.PollInterval); err != nil {
			return fmt.Errorf("poll_interval 格式无效，请使用如 2s、1m")
		}
	}
	return nil
}

func validatePipeline(_ PipelineSettings) error {
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

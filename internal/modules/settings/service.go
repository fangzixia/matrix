// Package settings 管理系统级运行参数：按业务域分表存储，启动时加载并热更新内存配置。
package settings

import (
	"context"
	"encoding/json"
	"fmt"
	"matrix/internal/platform/config"
	"matrix/internal/platform/db/repo"
	"matrix/internal/platform/gitutil"
	"matrix/internal/platform/logging"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ModelProfileSettings 单个 LLM 模型配置（含 API Key 脱敏字段）。
type ModelProfileSettings struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	BaseURL         string   `json:"base_url"`
	APIKey          string   `json:"api_key,omitempty"`
	APIKeySet       bool     `json:"api_key_set,omitempty"`
	Model           string   `json:"model"`
	MaxTokens       int      `json:"max_tokens"`
	Enabled         bool     `json:"enabled"`
	Default         bool     `json:"default"`
	Multimodal      bool     `json:"multimodal"`
	AttachmentTypes []string `json:"attachment_types"`
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

// Hooks 配置变更回调，用于同步 Git 到依赖服务。
type Hooks struct {
	OnGitUpdate func(config.GitConfig)
}

// Service 系统配置读写：DB 持久化 + 内存 runtime 热更新。
type Service struct {
	stores  *repo.Stores
	runtime *config.RuntimeConfig
	hooks   Hooks
	mu      sync.Mutex
}

// NewService 创建系统配置服务实例。
func NewService(stores *repo.Stores, runtime *config.RuntimeConfig) *Service {
	return &Service{stores: stores, runtime: runtime}
}

// Runtime 返回当前进程内运行时配置（只读引用，由 apply 热更新）。
func (s *Service) Runtime() *config.RuntimeConfig {
	return s.runtime
}

// SetHooks 注册配置变更回调，用于同步 Git 到依赖服务。
func (s *Service) SetHooks(h Hooks) {
	s.hooks = h
}

// Bootstrap 启动时从数据库按域加载并应用到内存配置；数据库无记录时使用系统默认值。
func (s *Service) Bootstrap(ctx context.Context) error {
	stored, err := s.loadAllStored(ctx)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if stored == nil {
		s.apply(defaultSettings(), true)
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
	def := defaultSettings()
	return &AISettings{
		Models: def.Models, Context: def.Context, Security: def.Security,
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
	if err := s.ReloadRuntime(ctx); err != nil {
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
	full := defaultSettings()
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
	if err := s.ReloadRuntime(ctx); err != nil {
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
	full := defaultSettings()
	git := full.Git
	normalizeGitSettings(&git)
	enrichGitHints(&git)
	return &git, nil
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
	if err := s.ReloadRuntime(ctx); err != nil {
		return nil, err
	}
	return s.GetGit(ctx)
}

// TestGit 测试 Git 连通性。
func (s *Service) TestGit(ctx context.Context, gitURL string) (string, error) {
	timeout := s.runtime.Git.CloneTimeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return gitutil.TestConnection(ctx, s.runtime.Git, gitURL, timeout)
}

// --- 存储层 ---

// saveDomain 将指定域配置序列化写入 system_settings 表。
func (s *Service) saveDomain(ctx context.Context, domainID string, data any) error {
	return s.stores.Settings.SaveDomain(ctx, domainID, data)
}

// loadDomainRaw 从数据库读取指定域的原始 JSON。
func (s *Service) loadDomainRaw(ctx context.Context, domainID string) (json.RawMessage, error) {
	return s.stores.Settings.LoadDomainRaw(ctx, domainID)
}

// loadDomainAI 加载 AI 域配置。
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

// loadDomainMCP 加载 MCP 域配置。
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

// loadDomainGit 加载 Git 域配置。
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

// loadAllStored 从数据库加载全部系统配置域。
func (s *Service) loadAllStored(ctx context.Context) (*Settings, error) {
	ai, _ := s.loadDomainAI(ctx)
	mcp, _ := s.loadDomainMCP(ctx)
	git, _ := s.loadDomainGit(ctx)
	if ai == nil && mcp == nil && git == nil {
		return nil, nil
	}
	base := defaultSettings()
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
	return &base, nil
}

// ReloadRuntime 从数据库重新加载全部系统配置并热更新内存 runtime（Save* 后调用）。
func (s *Service) ReloadRuntime(ctx context.Context) error {
	stored, err := s.loadAllStored(ctx)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if stored != nil {
		s.apply(*stored, false)
	}
	s.logAIReload()
	return nil
}

// ReloadAI 从数据库重新加载 AI 域配置（含 API Key），供 Run 启动前刷新 stale 内存。
func (s *Service) ReloadAI(ctx context.Context) error {
	stored, err := s.loadDomainAI(ctx)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if stored != nil {
		s.applyAI(*stored)
	}
	s.logAIReload()
	return nil
}

func (s *Service) logAIReload() {
	if p, ok := s.runtime.AI.ActiveModelProfile(); ok {
		logging.Info("settings: 已从数据库重载 AI 运行时配置",
			"default_model", p.Name,
			"model", p.Model,
			"model_count", len(s.runtime.AI.Models),
		)
		return
	}
	logging.Info("settings: AI runtime reloaded from database", "model_count", len(s.runtime.AI.Models))
}

// decorateAIForGet 为 API 返回的 AI 配置脱敏 API Key。
func (s *Service) decorateAIForGet(*AISettings) {}

// mergeAIInput 合并用户提交的 AI 配置与存量密钥。
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

func (s *Service) applyAI(st AISettings) {
	normalizeModelProfiles(&st.Models)
	s.runtime.AI.Models = toConfigModels(st.Models)
	if st.Context.AutoCompactThreshold > 0 {
		s.runtime.AI.Context.AutoCompactThreshold = st.Context.AutoCompactThreshold
	}
	if st.Context.KeepRecentMessages > 0 {
		s.runtime.AI.Context.KeepRecentMessages = st.Context.KeepRecentMessages
	}
	s.runtime.AI.Security.AllowShell = st.Security.AllowShell
	s.runtime.AI.Security.AllowCommandMCP = st.Security.AllowCommandMCP
	if d, err := time.ParseDuration(st.Security.ShellTimeout); err == nil && d > 0 {
		s.runtime.AI.Security.ShellTimeout = d
	}
}

// apply 应用。
func (s *Service) apply(st Settings, _ bool) {
	s.applyAI(AISettings{Models: st.Models, Context: st.Context, Security: st.Security})
	if st.MCPServers != nil {
		s.runtime.MCP.Servers = toMCPServers(st.MCPServers)
	} else {
		s.runtime.MCP.Servers = map[string]config.MCPServerConfig{}
	}
	s.runtime.Git = toGitConfig(st.Git)
	if d, err := time.ParseDuration(st.Git.CloneTimeout); err == nil && d > 0 {
		s.runtime.Git.CloneTimeout = d
	}
	if s.hooks.OnGitUpdate != nil {
		s.hooks.OnGitUpdate(s.runtime.Git)
	}
}

// normalizeGitSettings 规范化 Git 设置字段。
func normalizeGitSettings(g *GitSettings) {
	if len(g.Accesses) == 0 && g.SSHKeyPath != "" {
		g.Accesses = []GitAccess{{
			ID: "default", Name: "默认", Host: "*", SSHKeyPath: g.SSHKeyPath,
		}}
	}
}

// toGitConfig 将 GitSettings 转换为 config.GitConfig。
func toGitConfig(g GitSettings) config.GitConfig {
	normalizeGitSettings(&g)
	out := config.GitConfig{SSHKeyPath: g.SSHKeyPath}
	if d, err := time.ParseDuration(g.CloneTimeout); err == nil && d > 0 {
		out.CloneTimeout = d
	}
	for _, a := range g.Accesses {
		out.Accesses = append(out.Accesses, config.GitAccess{
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

// fromGitConfig 将 config.GitConfig 转换为 GitSettings。
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

// enrichGitHints 为 Git 设置补充 UI 提示字段。
func enrichGitHints(g *GitSettings) {
	g.Platform = gitutil.ServerPlatform()
	g.PlatformLabel = gitutil.PlatformLabel(g.Platform)
	g.DefaultSSHKeyPath = gitutil.DefaultSSHKeyPath()
}

// stripGitHints 移除 Git 设置中的 UI 提示字段。
func stripGitHints(g *GitSettings) {
	g.Platform = ""
	g.PlatformLabel = ""
	g.DefaultSSHKeyPath = ""
}

// validateAI 校验 AI 域配置合法性。
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

// validateGit 校验 Git 域配置合法性。
func validateGit(in GitSettings) error {
	if in.CloneTimeout != "" {
		if _, err := time.ParseDuration(in.CloneTimeout); err != nil {
			return fmt.Errorf("clone_timeout 格式无效，请使用如 300s、5m")
		}
	}
	return nil
}

// toMCPServers 将 MCPServerSettings 映射转换为 config 结构。
func toMCPServers(in map[string]MCPServerSettings) map[string]config.MCPServerConfig {
	out := make(map[string]config.MCPServerConfig, len(in))
	for name, srv := range in {
		out[name] = config.MCPServerConfig{
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

// fromMCPServers 将 config MCP 映射转换为 MCPServerSettings。
func fromMCPServers(in map[string]config.MCPServerConfig) map[string]MCPServerSettings {
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

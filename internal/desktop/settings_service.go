package desktop

import "matrix/internal/mcp"

// SettingsService owns config DTO conversion and persistence.
type SettingsService struct {
	config *Config
}

func NewSettingsService(cfg *Config) *SettingsService {
	return &SettingsService{config: cfg}
}

func (s *SettingsService) Get() *Settings {
	return s.config.ToSettings()
}

func (s *SettingsService) Save(settings *Settings) (map[string]mcp.ServerConfig, error) {
	s.config.FromSettings(settings)
	if err := SaveConfig(s.config); err != nil {
		return nil, wrapInternal("保存配置失败", err)
	}
	return mcpConfigsFromSettings(settings), nil
}

func mcpConfigsFromSettings(settings *Settings) map[string]mcp.ServerConfig {
	if settings == nil || settings.MCPServers == nil {
		return nil
	}
	configs := make(map[string]mcp.ServerConfig)
	for name, serverCfg := range settings.MCPServers {
		configs[name] = mcp.ServerConfig{
			Command:     serverCfg.Command,
			Args:        serverCfg.Args,
			Env:         serverCfg.Env,
			URL:         serverCfg.URL,
			Headers:     serverCfg.Headers,
			Disabled:    serverCfg.Disabled,
			AutoApprove: serverCfg.AutoApprove,
		}
	}
	return configs
}

func mcpConfigsFromConfig(cfg *Config) map[string]mcp.ServerConfig {
	if cfg == nil || cfg.MCPServers == nil {
		return nil
	}
	settings := &Settings{MCPServers: cfg.MCPServers}
	return mcpConfigsFromSettings(settings)
}

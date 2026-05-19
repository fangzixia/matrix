package desktop

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestMCPServerConfig(t *testing.T) {
	// 测试本地服务器配置
	localServer := MCPServerConfig{
		Command:     "uvx",
		Args:        []string{"mcp-server-filesystem@latest"},
		Env:         map[string]string{"DEBUG": "true"},
		Disabled:    false,
		AutoApprove: []string{"read_file", "list_directory"},
	}

	// 测试远程服务器配置
	remoteServer := MCPServerConfig{
		URL:         "https://api.example.com/mcp",
		Headers:     map[string]string{"Authorization": "Bearer token"},
		Disabled:    false,
		AutoApprove: []string{},
	}

	// 创建配置
	cfg := DefaultConfig()
	cfg.MCPServers = map[string]MCPServerConfig{
		"filesystem": localServer,
		"remote-api": remoteServer,
	}

	// 序列化为 JSON
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal config: %v", err)
	}

	// 反序列化
	var cfg2 Config
	if err := json.Unmarshal(data, &cfg2); err != nil {
		t.Fatalf("Failed to unmarshal config: %v", err)
	}

	// 验证 MCP 服务器配置
	if len(cfg2.MCPServers) != 2 {
		t.Errorf("Expected 2 MCP servers, got %d", len(cfg2.MCPServers))
	}

	// 验证本地服务器
	fs, ok := cfg2.MCPServers["filesystem"]
	if !ok {
		t.Fatal("filesystem server not found")
	}
	if fs.Command != "uvx" {
		t.Errorf("Expected command 'uvx', got '%s'", fs.Command)
	}
	if len(fs.Args) != 1 {
		t.Errorf("Expected 1 arg, got %d", len(fs.Args))
	}
	if len(fs.AutoApprove) != 2 {
		t.Errorf("Expected 2 auto-approve tools, got %d", len(fs.AutoApprove))
	}

	// 验证远程服务器
	remote, ok := cfg2.MCPServers["remote-api"]
	if !ok {
		t.Fatal("remote-api server not found")
	}
	if remote.URL != "https://api.example.com/mcp" {
		t.Errorf("Expected URL 'https://api.example.com/mcp', got '%s'", remote.URL)
	}
}

func TestConfigSaveLoad(t *testing.T) {
	// 创建临时目录
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	// 创建配置
	cfg := DefaultConfig()
	cfg.Model.APIKey = "test-key"
	cfg.MCPServers = map[string]MCPServerConfig{
		"test-server": {
			Command:     "test-command",
			Args:        []string{"arg1", "arg2"},
			Disabled:    false,
			AutoApprove: []string{"tool1"},
		},
	}

	// 保存配置
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal config: %v", err)
	}
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	// 加载配置
	data, err = os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config: %v", err)
	}

	var cfg2 Config
	if err := json.Unmarshal(data, &cfg2); err != nil {
		t.Fatalf("Failed to unmarshal config: %v", err)
	}

	// 验证配置
	if cfg2.Model.APIKey != "test-key" {
		t.Errorf("Expected API key 'test-key', got '%s'", cfg2.Model.APIKey)
	}

	testServer, ok := cfg2.MCPServers["test-server"]
	if !ok {
		t.Fatal("test-server not found")
	}
	if testServer.Command != "test-command" {
		t.Errorf("Expected command 'test-command', got '%s'", testServer.Command)
	}
	if len(testServer.Args) != 2 {
		t.Errorf("Expected 2 args, got %d", len(testServer.Args))
	}
}

func TestSettingsConversion(t *testing.T) {
	// 创建配置
	cfg := DefaultConfig()
	cfg.Model.APIKey = "test-key"
	cfg.MCPServers = map[string]MCPServerConfig{
		"server1": {
			Command:  "cmd1",
			Disabled: false,
		},
	}

	// 转换为 Settings
	settings := cfg.ToSettings()
	if settings.Model.APIKey != "test-key" {
		t.Errorf("Expected API key 'test-key', got '%s'", settings.Model.APIKey)
	}
	if len(settings.MCPServers) != 1 {
		t.Errorf("Expected 1 MCP server, got %d", len(settings.MCPServers))
	}

	// 从 Settings 更新配置
	cfg2 := DefaultConfig()
	settings.Model.APIKey = "new-key"
	settings.MCPServers = map[string]MCPServerConfig{
		"server2": {
			Command:  "cmd2",
			Disabled: true,
		},
	}
	cfg2.FromSettings(settings)

	if cfg2.Model.APIKey != "new-key" {
		t.Errorf("Expected API key 'new-key', got '%s'", cfg2.Model.APIKey)
	}
	if len(cfg2.MCPServers) != 1 {
		t.Errorf("Expected 1 MCP server, got %d", len(cfg2.MCPServers))
	}
	server2, ok := cfg2.MCPServers["server2"]
	if !ok {
		t.Fatal("server2 not found")
	}
	if !server2.Disabled {
		t.Error("Expected server2 to be disabled")
	}
}

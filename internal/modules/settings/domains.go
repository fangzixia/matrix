package settings

const (
	// DomainAI 是 AI 配置在 system_settings 表中的行 ID。
	DomainAI = "ai"
	// DomainMCP 是 MCP 配置在 system_settings 表中的行 ID。
	DomainMCP = "mcp"
	// DomainGit 是 Git 配置在 system_settings 表中的行 ID。
	DomainGit = "git"
)

// AISettings 模型、上下文与安全策略。
type AISettings struct {
	Models   []ModelProfileSettings `json:"models"`
	Context  ContextSettings        `json:"context"`
	Security SecuritySettings       `json:"security"`
}

// MCPSettings MCP 服务定义。
type MCPSettings struct {
	MCPServers map[string]MCPServerSettings `json:"mcp_servers"`
}

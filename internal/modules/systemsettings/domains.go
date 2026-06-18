package systemsettings

const (
	// DomainAI 是 AI 配置在 system_settings 表中的行 ID。
	DomainAI = "ai"
	// DomainMCP 是 MCP 配置在 system_settings 表中的行 ID。
	DomainMCP = "mcp"
	// DomainGit 是 Git 配置在 system_settings 表中的行 ID。
	DomainGit = "git"
	// DomainWorker 是 Worker 配置在 system_settings 表中的行 ID。
	DomainWorker = "worker"
	// DomainPipeline 是 Pipeline 配置在 system_settings 表中的行 ID。
	DomainPipeline = "pipeline"
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

// Settings 聚合视图（仅用于内存 apply，不整包持久化）。
type Settings struct {
	Models     []ModelProfileSettings       `json:"models"`
	Context    ContextSettings              `json:"context"`
	Security   SecuritySettings             `json:"security"`
	MCPServers map[string]MCPServerSettings `json:"mcp_servers"`
	Git        GitSettings                  `json:"git"`
	Worker     WorkerSettings               `json:"worker"`
	Pipeline   PipelineSettings             `json:"pipeline"`
}

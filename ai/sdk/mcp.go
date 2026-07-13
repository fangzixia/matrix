package sdk

import "matrix/ai/mcp"

// MCPManager 为 MCP 服务器管理器。
type MCPManager = mcp.Manager

// MCPServerConfig 为单个 MCP 服务器配置。
type MCPServerConfig = mcp.ServerConfig

var (
	// NewMCPManager 创建 MCP 管理器。
	NewMCPManager = mcp.NewManager
	// RegisterMCPTools 将 MCP 工具注册到工具表。
	RegisterMCPTools = mcp.RegisterTools
)

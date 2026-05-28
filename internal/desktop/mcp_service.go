package desktop

import (
	"fmt"

	"matrix/internal/logger"
	"matrix/internal/mcp"
)

// MCPService owns MCP manager interactions and frontend DTO mapping.
type MCPService struct {
	manager *mcp.Manager
}

func NewMCPService(manager *mcp.Manager) *MCPService {
	return &MCPService{manager: manager}
}

func (s *MCPService) UpdateConfigs(configs map[string]mcp.ServerConfig) {
	if s == nil || s.manager == nil || configs == nil {
		return
	}
	s.manager.UpdateConfigs(configs)
}

func (s *MCPService) AutoConnect() {
	if s == nil || s.manager == nil {
		return
	}
	statuses := s.manager.ReconnectAll()
	available := 0
	for _, st := range statuses {
		if st.Available {
			available++
		}
	}
	logger.Infof("MCP auto-connect: %d/%d servers available", available, len(statuses))
}

func (s *MCPService) TestServer(name string) *MCPServerStatus {
	return mcpStatusToFrontend(s.manager.TestServer(name))
}

func (s *MCPService) TestAllServers() map[string]*MCPServerStatus {
	return mcpStatusesToFrontend(s.manager.ReconnectAll())
}

func (s *MCPService) GetServerStatus(name string) *MCPServerStatus {
	return mcpStatusToFrontend(s.manager.GetServerStatus(name))
}

func (s *MCPService) GetAllServerStatuses() map[string]*MCPServerStatus {
	return mcpStatusesToFrontend(s.manager.GetAllServerStatuses())
}

func (s *MCPService) CallTool(serverName, toolName string, arguments map[string]interface{}) (map[string]interface{}, error) {
	result, err := s.manager.CallTool(serverName, toolName, arguments)
	if err != nil {
		return nil, fmt.Errorf("call tool: %w", err)
	}
	return map[string]interface{}{
		"isError": result.IsError,
		"content": result.Content,
	}, nil
}

func (s *MCPService) ListTools(serverName string) ([]map[string]interface{}, error) {
	tools, err := s.manager.ListTools(serverName)
	if err != nil {
		return nil, fmt.Errorf("list tools: %w", err)
	}
	return mcpToolsToFrontend(tools), nil
}

func (s *MCPService) ListAllTools() (map[string]interface{}, error) {
	allTools, err := s.manager.ListAllTools()
	if err != nil {
		return nil, fmt.Errorf("list all tools: %w", err)
	}
	result := make(map[string]interface{})
	for serverName, tools := range allTools {
		result[serverName] = mcpToolsToFrontend(tools)
	}
	return result, nil
}

func mcpStatusToFrontend(status *mcp.ServerStatus) *MCPServerStatus {
	if status == nil {
		return &MCPServerStatus{}
	}
	return &MCPServerStatus{
		Name:       status.Name,
		Available:  status.Available,
		ToolCount:  status.ToolCount,
		Tools:      status.Tools,
		Error:      status.Error,
		LastTested: status.LastTested,
	}
}

func mcpStatusesToFrontend(statuses map[string]*mcp.ServerStatus) map[string]*MCPServerStatus {
	results := make(map[string]*MCPServerStatus)
	for name, status := range statuses {
		results[name] = mcpStatusToFrontend(status)
	}
	return results
}

func mcpToolsToFrontend(tools []mcp.Tool) []map[string]interface{} {
	result := make([]map[string]interface{}, len(tools))
	for i, tool := range tools {
		result[i] = map[string]interface{}{
			"name":        tool.Name,
			"description": tool.Description,
			"inputSchema": tool.InputSchema,
		}
	}
	return result
}

package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"matrix/internal/ai/mcp"
	"matrix/internal/platform/logging"
)

// NewMCPTool 将 MCP 工具包装为可注册到 Registry 的 Tool。
func NewMCPTool(serverName string, tool mcp.Tool, manager *mcp.Manager) *Tool {
	name := fmt.Sprintf("mcp_%s_%s", serverName, tool.Name)
	desc := tool.Description
	if desc == "" {
		desc = fmt.Sprintf("MCP tool from %s server", serverName)
	}
	description := fmt.Sprintf("[MCP:%s] %s", serverName, desc)
	params := convertMCPSchema(tool.InputSchema)
	execute := func(ctx context.Context, args map[string]any) (string, error) {
		logging.Infof("Calling MCP tool: server=%s, tool=%s", serverName, tool.Name)
		result, err := manager.CallTool(serverName, tool.Name, args)
		if err != nil {
			return "", fmt.Errorf("call MCP tool: %w", err)
		}
		if result.IsError {
			return "", fmt.Errorf("MCP tool error: %s", formatContent(result.Content))
		}
		return formatContent(result.Content), nil
	}
	return &Tool{
		Name:              name,
		Description:       description,
		Parameters:        params,
		IsConcurrencySafe: true,
		Execute:           execute,
	}
}

// convertMCPSchema 将 MCP JSON Schema 转换为工具参数定义。
func convertMCPSchema(inputSchema mcp.InputSchema) JSONSchema {
	schema := JSONSchema{
		Type:       inputSchema.Type,
		Properties: make(map[string]PropSchema),
		Required:   inputSchema.Required,
	}
	if schema.Type == "" {
		schema.Type = "object"
	}
	for propName, propValue := range inputSchema.Properties {
		schema.Properties[propName] = convertProperty(propValue)
	}
	return schema
}

// convertProperty 转换 MCP Schema 单个属性定义。
func convertProperty(prop interface{}) PropSchema {
	propSchema := PropSchema{}
	if propMap, ok := prop.(map[string]interface{}); ok {
		if t, ok := propMap["type"].(string); ok {
			propSchema.Type = t
		}
		if desc, ok := propMap["description"].(string); ok {
			propSchema.Description = desc
		}
		if props, ok := propMap["properties"].(map[string]interface{}); ok {
			propSchema.Properties = make(map[string]PropSchema)
			for k, v := range props {
				propSchema.Properties[k] = convertProperty(v)
			}
		}
		if items, ok := propMap["items"].(map[string]interface{}); ok {
			propSchema.Items = new(convertProperty(items))
		}
		if required, ok := propMap["required"].([]interface{}); ok {
			propSchema.Required = make([]string, len(required))
			for i, r := range required {
				if s, ok := r.(string); ok {
					propSchema.Required[i] = s
				}
			}
		}
	}
	return propSchema
}

// formatContent 格式化 MCP 工具返回内容为字符串。
func formatContent(contents []mcp.Content) string {
	if len(contents) == 0 {
		return ""
	}
	var result string
	for i, content := range contents {
		if i > 0 {
			result += "\n\n"
		}
		switch content.Type {
		case "text":
			result += content.Text
		case "image":
			result += fmt.Sprintf("[Image: %s]", content.MimeType)
		case "resource":
			result += fmt.Sprintf("[Resource]")
		default:
			data, err := json.Marshal(content)
			if err == nil {
				result += string(data)
			}
		}
	}
	return result
}

// RegisterMCPTools 将 Manager 发现的全部 MCP 工具注册到 Registry。
func RegisterMCPTools(registry *Registry, manager *mcp.Manager) error {
	allTools, err := manager.ListAllTools()
	if err != nil {
		return fmt.Errorf("list all tools: %w", err)
	}
	for serverName, tools := range allTools {
		for _, tool := range tools {
			mcpTool := NewMCPTool(serverName, tool, manager)
			registry.Register(mcpTool)
			logging.Infof("Registered MCP tool: %s", mcpTool.Name)
		}
	}
	return nil
}

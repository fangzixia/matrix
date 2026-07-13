package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	aiutil "matrix/ai/util"
)

// NewTool 将 MCP 工具包装为可注册到 util.Registry 的 Tool。
func NewTool(serverName string, mcpTool Tool, manager *Manager) *aiutil.Tool {
	name := fmt.Sprintf("mcp_%s_%s", serverName, mcpTool.Name)
	desc := mcpTool.Description
	if desc == "" {
		desc = fmt.Sprintf("MCP tool from %s server", serverName)
	}
	description := fmt.Sprintf("[MCP:%s] %s", serverName, desc)
	params := convertSchema(mcpTool.InputSchema)
	execute := func(ctx context.Context, args map[string]any) (string, error) {
		argsJSON, _ := json.Marshal(args)
		slog.InfoContext(ctx, "loop: MCP 工具调用",
			"server", serverName,
			"tool", mcpTool.Name,
			"input", string(argsJSON),
		)
		result, err := manager.CallTool(serverName, mcpTool.Name, args)
		if err != nil {
			return "", fmt.Errorf("call MCP tool: %w", err)
		}
		if result.IsError {
			out := formatContent(result.Content)
			slog.InfoContext(ctx, "loop: MCP 工具结果",
				"server", serverName,
				"tool", mcpTool.Name,
				"is_error", true,
				"output", out,
			)
			return "", fmt.Errorf("MCP tool error: %s", out)
		}
		out := formatContent(result.Content)
		slog.InfoContext(ctx, "loop: MCP 工具结果",
			"server", serverName,
			"tool", mcpTool.Name,
			"is_error", false,
			"output", out,
		)
		aiutil.WriteChunks(ctx, out, aiutil.DefaultEmitChunkSize)
		return out, nil
	}
	return &aiutil.Tool{
		Name:              name,
		Description:       description,
		Parameters:        params,
		IsConcurrencySafe: true,
		StatusLabel: func(_ map[string]any) string {
			return fmt.Sprintf("MCP %s/%s …", serverName, mcpTool.Name)
		},
		Execute: execute,
	}
}

func convertSchema(inputSchema InputSchema) aiutil.JSONSchema {
	schema := aiutil.JSONSchema{
		Type:       inputSchema.Type,
		Properties: make(map[string]aiutil.PropSchema),
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

func convertProperty(prop interface{}) aiutil.PropSchema {
	propSchema := aiutil.PropSchema{}
	if propMap, ok := prop.(map[string]interface{}); ok {
		if t, ok := propMap["type"].(string); ok {
			propSchema.Type = t
		}
		if desc, ok := propMap["description"].(string); ok {
			propSchema.Description = desc
		}
		if props, ok := propMap["properties"].(map[string]interface{}); ok {
			propSchema.Properties = make(map[string]aiutil.PropSchema)
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

func formatContent(contents []Content) string {
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
			result += "[Resource]"
		default:
			data, err := json.Marshal(content)
			if err == nil {
				result += string(data)
			}
		}
	}
	return result
}

// RegisterTools 将 Manager 发现的全部 MCP 工具注册到 Registry（Extension）。
func RegisterTools(registry *aiutil.Registry, manager *Manager) error {
	allTools, err := manager.ListAllTools()
	if err != nil {
		return fmt.Errorf("list all tools: %w", err)
	}
	for serverName, serverTools := range allTools {
		for _, t := range serverTools {
			mcpTool := NewTool(serverName, t, manager)
			registry.Register(mcpTool)
			slog.Info("Registered MCP tool", "name", mcpTool.Name)
		}
	}
	return nil
}

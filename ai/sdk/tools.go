package sdk

import (
	"matrix/ai/tools"
	"matrix/ai/util"
)

// Tool 为可注册的工具定义。
//
// 工具流式输出请写 util.StreamWriter(ctx)；TOOL_CALL_* 生命周期由 util.RunTools 负责。
// 自定义工具勿在 Execute 内直接推送 AG-UI 事件。
type Tool = util.Tool

// ToolRegistry 为工具注册表。
type ToolRegistry = util.Registry

// JSONSchema 为工具参数 JSON Schema。
type JSONSchema = util.JSONSchema

// PropSchema 为 JSON Schema 属性描述。
type PropSchema = util.PropSchema

// ToolResult 为单次工具调用结果。
type ToolResult = util.Result

var (
	// NewToolRegistry 创建空工具注册表。
	NewToolRegistry = util.NewRegistry
	// DefaultRegistry 返回含全部内置工具的注册表。
	DefaultRegistry = tools.DefaultRegistry
	// RegistryWithoutShell 返回不含 bash/powershell 的内置工具注册表。
	RegistryWithoutShell = tools.RegistryWithoutShell
)

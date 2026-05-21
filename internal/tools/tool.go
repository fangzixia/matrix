// Package tools 提供工具的注册、执行调度和并发管理功能。
// 只读工具（IsConcurrencySafe=true）批量并行执行；写工具串行执行以保证顺序一致性。
package tools

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"matrix/internal/llm"
	"matrix/internal/stream"
)

// JSONSchema 是工具参数的最小化 JSON Schema 描述。
type JSONSchema struct {
	// Type 为 Schema 根类型，通常为 "object"。
	Type       string                `json:"type"`
	Properties map[string]PropSchema `json:"properties,omitempty"`
	Required   []string              `json:"required,omitempty"`
}

// PropSchema 描述 [JSONSchema.Properties] 中的单个属性，
// 或数组元素的 object/items 结构（对标 JSON Schema 子集）。
type PropSchema struct {
	Type        string                `json:"type"`
	Description string                `json:"description,omitempty"`
	Properties  map[string]PropSchema `json:"properties,omitempty"`
	Items       *PropSchema           `json:"items,omitempty"`
	Required    []string              `json:"required,omitempty"`
}

// Tool 是可注册到 Agent 的单个可调用能力单元。
type Tool struct {
	// Name 必须与 LLM 在 tool_calls[].function.name 中输出的名称完全一致。
	Name string
	// Description 展示给 LLM，帮助其决定何时调用该工具。
	Description string
	// Parameters 描述工具输入参数的 JSON Schema。
	Parameters JSONSchema
	// IsConcurrencySafe 为 true 表示工具无可观测副作用，
	// 可与同批次其他安全工具并发执行。
	IsConcurrencySafe bool
	// Execute 执行工具逻辑，返回输出字符串；失败时返回非 nil 错误。
	Execute func(ctx context.Context, args map[string]any) (string, error)
}

// propSchemaToMap 将嵌套 PropSchema 转为 OpenAI tools.parameters.properties 所用的 map。
func propSchemaToMap(p PropSchema) map[string]any {
	m := map[string]any{
		"type": p.Type,
	}
	if p.Description != "" {
		m["description"] = p.Description
	}
	if len(p.Properties) > 0 {
		props := make(map[string]any, len(p.Properties))
		for k, v := range p.Properties {
			props[k] = propSchemaToMap(v)
		}
		m["properties"] = props
	}
	if p.Items != nil {
		m["items"] = propSchemaToMap(*p.Items)
	}
	if len(p.Required) > 0 {
		m["required"] = p.Required
	}
	return m
}

// ToLLMTool 将 Tool 转换为 [llm.Tool]，用于构造 API 请求体。
func (t *Tool) ToLLMTool() llm.Tool {
	params := map[string]any{
		"type": t.Parameters.Type,
	}
	if len(t.Parameters.Properties) > 0 {
		props := make(map[string]any, len(t.Parameters.Properties))
		for k, v := range t.Parameters.Properties {
			props[k] = propSchemaToMap(v)
		}
		params["properties"] = props
	}
	if len(t.Parameters.Required) > 0 {
		params["required"] = t.Parameters.Required
	}
	return llm.Tool{
		Type: "function",
		Function: llm.ToolFunction{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  params,
		},
	}
}

// Result 保存单次工具调用的执行结果。
type Result struct {
	// ToolCallID 对应 [llm.ToolCall.ID]，用于关联请求与结果。
	ToolCallID string
	// ToolName 为工具名称，便于日志和调试。
	ToolName string
	// Output 为工具返回的文本输出；IsError 为 true 时为错误描述。
	Output string
	// IsError 为 true 表示工具执行失败。
	IsError bool
}

// Registry 将工具名映射到 [Tool] 定义。
type Registry struct {
	tools map[string]*Tool
}

// NewRegistry 创建并返回一个空的 Registry。
func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]*Tool)}
}

// Register 向注册表中添加一个工具。若名称已存在则 panic。
func (r *Registry) Register(t *Tool) {
	if _, exists := r.tools[t.Name]; exists {
		panic(fmt.Sprintf("tools: 重复的工具名 %q", t.Name))
	}
	r.tools[t.Name] = t
}

// Get 按名称查找工具，不存在时返回 nil。
func (r *Registry) Get(name string) *Tool {
	return r.tools[name]
}

// Names 返回已注册的所有工具名称切片（无序）。
func (r *Registry) Names() []string {
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	return names
}

// LLMTools 返回所有已注册工具的 [llm.Tool] 切片，供构造 API 请求使用。
func (r *Registry) LLMTools() []llm.Tool {
	out := make([]llm.Tool, 0, len(r.tools))
	for _, t := range r.tools {
		out = append(out, t.ToLLMTool())
	}
	return out
}

// CanUseToolFn 是工具执行前的权限检查回调。返回 false 则拒绝执行。
type CanUseToolFn func(toolName string, args map[string]any) bool

// ProgressFn 报告工具执行进度（started / completed / failed）。
type ProgressFn func(toolUseID string, data stream.ToolProgressData)

// RunTools 执行 calls 中的所有工具调用，按并发安全性自动分批。
func RunTools(
	ctx context.Context,
	calls []llm.ToolCall,
	reg *Registry,
	canUse CanUseToolFn,
	onProgress ProgressFn,
) []Result {
	batches := partitionCalls(calls, reg)
	var all []Result
	for _, b := range batches {
		if b.parallel {
			all = append(all, runParallel(ctx, b.calls, reg, canUse, onProgress)...)
		} else {
			all = append(all, runSerial(ctx, b.calls, reg, canUse, onProgress)...)
		}
	}
	return all
}

// batch 是内部调度单元，记录一批调用是否可以并行执行。
type batch struct {
	parallel bool
	calls    []llm.ToolCall
}

// partitionCalls 将 calls 按并发安全性分组：连续的安全调用合并为一个并行批次，
// 不安全的调用各自成为单独的串行批次。
func partitionCalls(calls []llm.ToolCall, reg *Registry) []batch {
	var batches []batch
	for _, c := range calls {
		t := reg.Get(c.Function.Name)
		safe := t != nil && t.IsConcurrencySafe
		last := len(batches) - 1
		if safe && last >= 0 && batches[last].parallel {
			batches[last].calls = append(batches[last].calls, c)
		} else {
			batches = append(batches, batch{parallel: safe, calls: []llm.ToolCall{c}})
		}
	}
	return batches
}

// runParallel 使用 sync.WaitGroup 并发执行 calls 中的所有工具调用。
func runParallel(
	ctx context.Context,
	calls []llm.ToolCall,
	reg *Registry,
	canUse CanUseToolFn,
	onProgress ProgressFn,
) []Result {
	results := make([]Result, len(calls))
	var wg sync.WaitGroup
	for i, c := range calls {
		wg.Add(1)
		go func(idx int, call llm.ToolCall) {
			defer wg.Done()
			results[idx] = execOne(ctx, call, reg, canUse, onProgress)
		}(i, c)
	}
	wg.Wait()
	return results
}

func runSerial(
	ctx context.Context,
	calls []llm.ToolCall,
	reg *Registry,
	canUse CanUseToolFn,
	onProgress ProgressFn,
) []Result {
	results := make([]Result, 0, len(calls))
	for _, c := range calls {
		results = append(results, execOne(ctx, c, reg, canUse, onProgress))
	}
	return results
}

func execOne(
	ctx context.Context,
	call llm.ToolCall,
	reg *Registry,
	canUse CanUseToolFn,
	onProgress ProgressFn,
) Result {
	args, err := parseArgs(call.Function.Arguments)
	if err != nil {
		out := fmt.Sprintf("参数解析失败: %v", err)
		emitToolDone(onProgress, call, "failed", 0, out)
		return Result{
			ToolCallID: call.ID,
			ToolName:   call.Function.Name,
			Output:     out,
			IsError:    true,
		}
	}

	if canUse != nil && !canUse(call.Function.Name, args) {
		out := fmt.Sprintf("permission denied: %s", call.Function.Name)
		emitToolDone(onProgress, call, "failed", 0, out)
		return Result{
			ToolCallID: call.ID,
			ToolName:   call.Function.Name,
			Output:     out,
			IsError:    true,
		}
	}

	t := reg.Get(call.Function.Name)
	if t == nil {
		out := fmt.Sprintf("unknown tool: %s", call.Function.Name)
		emitToolDone(onProgress, call, "failed", 0, out)
		return Result{
			ToolCallID: call.ID,
			ToolName:   call.Function.Name,
			Output:     out,
			IsError:    true,
		}
	}

	emitProgress(onProgress, call.ID, stream.ToolProgressData{
		Type:     stream.DataToolProgress,
		Status:   "started",
		ToolName: call.Function.Name,
		Message:  call.Function.Arguments,
	})
	if strings.HasPrefix(call.Function.Name, "mcp_") {
		server, tool := parseMCPToolName(call.Function.Name)
		emitProgress(onProgress, call.ID, stream.ToolProgressData{
			Type:       stream.DataMCPProgress,
			Status:     "started",
			ServerName: server,
			ToolName:   tool,
		})
	}

	start := time.Now()
	output, execErr := t.Execute(ContextWithToolCallID(ctx, call.ID), args)
	elapsed := time.Since(start).Milliseconds()
	if execErr != nil {
		out := execErr.Error()
		if output != "" {
			out = output + "\n\n[执行错误] " + execErr.Error()
		}
		emitToolDone(onProgress, call, "failed", elapsed, out)
		return Result{
			ToolCallID: call.ID,
			ToolName:   call.Function.Name,
			Output:     out,
			IsError:    true,
		}
	}
	emitToolDone(onProgress, call, "completed", elapsed, output)
	return Result{
		ToolCallID: call.ID,
		ToolName:   call.Function.Name,
		Output:     output,
	}
}

func emitProgress(fn ProgressFn, toolUseID string, data stream.ToolProgressData) {
	if fn == nil || toolUseID == "" {
		return
	}
	fn(toolUseID, data)
}

const toolOutputPreviewMax = 500

func previewToolOutput(s string) string {
	if s == "" {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= toolOutputPreviewMax {
		return s
	}
	return string(runes[:toolOutputPreviewMax]) + "…"
}

func emitToolDone(fn ProgressFn, call llm.ToolCall, status string, elapsedMs int64, output string) {
	if fn == nil {
		return
	}
	preview := previewToolOutput(output)
	emitProgress(fn, call.ID, stream.ToolProgressData{
		Type:          stream.DataToolProgress,
		Status:        status,
		ToolName:      call.Function.Name,
		ElapsedTimeMs: elapsedMs,
		Message:       preview,
	})
	if strings.HasPrefix(call.Function.Name, "mcp_") {
		server, tool := parseMCPToolName(call.Function.Name)
		emitProgress(fn, call.ID, stream.ToolProgressData{
			Type:          stream.DataMCPProgress,
			Status:        status,
			ServerName:    server,
			ToolName:      tool,
			ElapsedTimeMs: elapsedMs,
			Message:       preview,
		})
	}
}

func parseMCPToolName(name string) (server, tool string) {
	if !strings.HasPrefix(name, "mcp_") {
		return "", name
	}
	rest := strings.TrimPrefix(name, "mcp_")
	parts := strings.SplitN(rest, "_", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return rest, ""
}

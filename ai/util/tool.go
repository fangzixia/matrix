// Package util 提供 Tool 类型、Registry 与 RunTools 执行调度。
// 只读工具（IsConcurrencySafe=true）批量并行执行；写工具串行执行以保证顺序一致性。
package util

import (
	"context"
	"fmt"
	"io"
	"matrix/ai/llm"
	"matrix/ai/stream"
	"sync"
	"time"
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
	// StatusLabel 可选；execOne 在 TOOL_CALL_START 后写入一行开始文案（末尾自动补 \n）。
	// 返回空字符串时使用默认「正在调用 {name} …」。
	StatusLabel func(args map[string]any) string
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
	tools           map[string]*Tool
	unknownToolHint func(name string) string
}

// NewRegistry 创建并返回一个空的 Registry。
func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]*Tool)}
}

// SetUnknownToolHint 设置未知工具名时的自定义提示（如父 Agent 误调 Worker 工具）。
func (r *Registry) SetUnknownToolHint(fn func(name string) string) {
	r.unknownToolHint = fn
}

// Register 向注册表中添加一个工具。若名称已存在则 panic。
func (r *Registry) Register(t *Tool) {
	if _, exists := r.tools[t.Name]; exists {
		panic(fmt.Sprintf("util: 重复的工具名 %q", t.Name))
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

// ProgressFn 报告工具执行进度（AG-UI 事件）。
type ProgressFn func(ev stream.Event)

// RunTools 执行 calls 中的所有工具调用，按并发安全性自动分批。
func RunTools(
	ctx context.Context,
	calls []llm.ToolCall,
	reg *Registry,
	canUse CanUseToolFn,
	onProgress ProgressFn,
	parentMessageID string,
) []Result {
	batches := partitionCalls(calls, reg)
	var all []Result
	for _, b := range batches {
		if b.parallel {
			all = append(all, runParallel(ctx, b.calls, reg, canUse, onProgress, parentMessageID)...)
		} else {
			all = append(all, runSerial(ctx, b.calls, reg, canUse, onProgress, parentMessageID)...)
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
	parentMessageID string,
) []Result {
	results := make([]Result, len(calls))
	var wg sync.WaitGroup
	for i, c := range calls {
		wg.Add(1)
		go func(idx int, call llm.ToolCall) {
			defer wg.Done()
			results[idx] = execOne(ctx, call, reg, canUse, onProgress, parentMessageID)
		}(i, c)
	}
	wg.Wait()
	return results
}

// runSerial 串行执行多个工具调用。
func runSerial(
	ctx context.Context,
	calls []llm.ToolCall,
	reg *Registry,
	canUse CanUseToolFn,
	onProgress ProgressFn,
	parentMessageID string,
) []Result {
	results := make([]Result, 0, len(calls))
	for _, c := range calls {
		results = append(results, execOne(ctx, c, reg, canUse, onProgress, parentMessageID))
	}
	return results
}

// execOne 执行单个工具调用并返回结果。
func execOne(
	ctx context.Context,
	call llm.ToolCall,
	reg *Registry,
	canUse CanUseToolFn,
	onProgress ProgressFn,
	parentMessageID string,
) Result {
	args, err := parseArgs(call.Function.Arguments)
	if err != nil {
		out := fmt.Sprintf("参数解析失败: %v", err)
		emitToolDone(onProgress, call, parentMessageID, "failed", 0, out, nil)
		return Result{
			ToolCallID: call.ID,
			ToolName:   call.Function.Name,
			Output:     out,
			IsError:    true,
		}
	}
	if canUse != nil && !canUse(call.Function.Name, args) {
		out := fmt.Sprintf("permission denied: %s", call.Function.Name)
		emitToolDone(onProgress, call, parentMessageID, "failed", 0, out, nil)
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
		if reg.unknownToolHint != nil {
			if hint := reg.unknownToolHint(call.Function.Name); hint != "" {
				out = hint
			}
		}
		emitToolDone(onProgress, call, parentMessageID, "failed", 0, out, nil)
		return Result{
			ToolCallID: call.ID,
			ToolName:   call.Function.Name,
			Output:     out,
			IsError:    true,
		}
	}
	emitEvent(onProgress, stream.ToolCallStart(call.ID, call.Function.Name))
	spill, _ := NewOutputSpillWriter(ctx, call.ID)
	execCtx := ContextWithToolCallID(ctx, call.ID)
	rep := &OutputReporter{
		ToolUseID: call.ID,
		ToolName:  call.Function.Name,
		Emit:      onProgress,
		Spill:     spill,
	}
	execCtx = ContextWithReporter(execCtx, rep)
	execCtx = WithStreamWriter(execCtx, newReporterWriter(execCtx, rep))
	label := ""
	if t.StatusLabel != nil {
		label = t.StatusLabel(args)
	}
	if label == "" {
		label = fmt.Sprintf("正在调用 %s …", t.Name)
	}
	_, _ = io.WriteString(StreamWriter(execCtx), label+"\n")
	start := time.Now()
	output, execErr := t.Execute(execCtx, args)
	elapsed := time.Since(start).Milliseconds()
	if execErr != nil {
		out := execErr.Error()
		if output != "" {
			out = output + "\n\n[执行错误] " + execErr.Error()
		}
		emitToolDone(onProgress, call, parentMessageID, "failed", elapsed, out, spill)
		return Result{
			ToolCallID: call.ID,
			ToolName:   call.Function.Name,
			Output:     out,
			IsError:    true,
		}
	}
	emitToolDone(onProgress, call, parentMessageID, "completed", elapsed, output, spill)
	return Result{
		ToolCallID: call.ID,
		ToolName:   call.Function.Name,
		Output:     output,
	}
}

// emitEvent 推送 AG-UI 工具进度事件。
func emitEvent(fn ProgressFn, ev stream.Event) {
	if fn == nil || ev == nil {
		return
	}
	fn(ev)
}

const toolOutputPreviewMax = 500

// previewToolOutput 生成工具输出的审计预览。
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

// emitToolDone 推送工具执行完成事件。
func emitToolDone(fn ProgressFn, call llm.ToolCall, parentMessageID, status string, elapsedMs int64, output string, spill *OutputSpillWriter) {
	if fn == nil {
		return
	}
	preview := previewToolOutput(output)
	if spill != nil && spill.Path() != "" {
		preview = appendSpillHint(preview, spill.Path())
	}
	emitEvent(fn, stream.ToolCallEnd(call.ID))
	if preview == "" {
		preview = status
	}
	emitEvent(fn, stream.ToolCallResult(parentMessageID, call.ID, preview))
}

func appendSpillHint(preview, spillPath string) string {
	hint := "\n[full log: " + spillPath + "]"
	if preview == "" {
		return hint
	}
	return preview + hint
}

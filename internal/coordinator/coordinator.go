// Package coordinator 提供多 Agent 编排工具，包括子 Agent 的派生、续接和停止。
//
// 与 claude-code 的对应关系：
//   - [WorkerContext]      ↔ coordinatorMode.ts getCoordinatorUserContext()
//   - [CoordinatorSystemPrompt] ↔ coordinatorMode.ts getCoordinatorSystemPrompt()
//   - [AsyncSupport]       ↔ runAgent.ts notifyOnCompletion + AppState 的任务队列
//   - [NewAgentTool]       ↔ AgentTool.tsx（配置了 Async 时异步执行子 Agent）
//   - [NewSendMessageTool] ↔ SendMessageTool.ts
//   - [NewTaskStopTool]    ↔ TaskStopTool
//
// 关键设计：
//   - AgentTool 始终注册；子 Agent 是否异步由 Coordinator Config.Async 是否非 nil 决定。
//   - 异步结果以 user-role 消息注入父 TAOR 循环（query.Config.AsyncResults）。
//
// 包依赖关系（无环）：
//
//	llm → (无)
//	tools → llm
//	query → tools + llm
//	agent → query
//	coordinator → agent + query + tools + llm  ← 本包
package coordinator

import (
	"context"
	"fmt"
	"matrix/internal/logger"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"matrix/internal/agent"
	"matrix/internal/llm"
	"matrix/internal/query"
	"matrix/internal/tools"
)

// WorkerContext 返回追加到系统提示词末尾的 Worker 工具上下文说明。
// 对应 claude-code 的 getCoordinatorUserContext()：
// 告知 Coordinator LLM Worker 可以使用哪些工具，无需修改 system prompt 主体。
func WorkerContext(toolNames []string) string {
	sorted := make([]string, len(toolNames))
	copy(sorted, toolNames)
	sort.Strings(sorted)
	return fmt.Sprintf(
		"Workers spawned via the agent tool have access to these tools: %s",
		strings.Join(sorted, ", "),
	)
}

// ── AsyncSupport：异步子 Agent 的通道 + 计数器 ───────────────────────────

// AsyncSupport 封装 Coordinator 异步子 Agent 执行所需的通道和原子计数器。
//
// 使用方式（main.go 中）：
//
//	async := coordinator.NewAsyncSupport()
//	coordCfg := coordinator.Config{Async: async, ...}
//	asyncResults, hasPending := async.QueryConfigFields()
//	queryCfg := query.Config{AsyncResults: asyncResults, HasPendingAsync: hasPending, ...}
type AsyncSupport struct {
	ch    chan query.Message
	count int32
}

// NewAsyncSupport 创建用于 Coordinator 异步执行的 [AsyncSupport]。
func NewAsyncSupport() *AsyncSupport {
	return &AsyncSupport{ch: make(chan query.Message, 64)}
}

// Inc 增加待完成的异步子 Agent 计数（在 goroutine 启动前调用）。
func (a *AsyncSupport) Inc() { atomic.AddInt32(&a.count, 1) }

// Dec 减少待完成的异步子 Agent 计数（在 goroutine 完成后调用）。
func (a *AsyncSupport) Dec() { atomic.AddInt32(&a.count, -1) }

// Send 将异步结果消息写入注入通道；阻塞直到通道有空间。
func (a *AsyncSupport) Send(msg query.Message) { a.ch <- msg }

// HasPending 返回当前是否还有未完成的异步子 Agent。
func (a *AsyncSupport) HasPending() bool { return atomic.LoadInt32(&a.count) > 0 }

// QueryConfigFields 返回适配 [query.Config] 的接收通道和 HasPendingAsync 函数。
// 返回值直接赋给 query.Config.AsyncResults 和 query.Config.HasPendingAsync。
func (a *AsyncSupport) QueryConfigFields() (<-chan query.Message, func() bool) {
	return a.ch, a.HasPending
}

// NewRegistry 创建子 Agent 状态注册表，是 [agent.NewRegistry] 的便捷包装。
func NewRegistry() *agent.Registry {
	return agent.NewRegistry()
}

// ── Config ────────────────────────────────────────────────────────────────

// Config 包含创建编排工具所需的参数，由 Coordinator 初始化时传入。
type Config struct {
	// LLM 为 OpenAI 兼容的对话客户端（必填）。
	LLM *llm.Client
	// Model 为传递给 LLM API 的模型名称。
	Model string
	// AgentRegistry 为子 Agent 状态注册表（必填）。
	AgentRegistry *agent.Registry
	// ToolRegistry 为 Worker 可用的工具集（传给 query.Config.Registry）。
	ToolRegistry *tools.Registry
	// CanUseTool 为 Worker 的权限检查回调；nil 表示允许所有工具。
	CanUseTool tools.CanUseToolFn
	// MaxTurns 为 Worker 的默认最大迭代轮次（0 表示不限）。
	MaxTurns int
	// MaxTokens 为每次 LLM 请求的 token 上限。
	MaxTokens int
	// ContextPolicy is inherited by worker query loops so coordinator, workers,
	// and resumed workers share the same context-management pipeline.
	ContextPolicy query.ContextPolicy
	// MaxToolResultRunes limits each worker tool result before it enters worker
	// history.
	MaxToolResultRunes int
	// Async 为异步子 Agent 支持；nil 时 [NewAgentTool] 在同步路径阻塞直至 Worker 结束。
	// 应由 main.go 通过 NewAsyncSupport() 创建并同时传给 query.Config。
	Async *AsyncSupport
}

// ── System Prompts ────────────────────────────────────────────────────────

// WorkerSystemPrompt 是默认的 Worker 系统提示词。
// Worker 无需了解 Coordinator 的存在，只需专注完成被委派的任务。
const WorkerSystemPrompt = `你是一个专注的 AI 工作单元（Worker Agent）。
你的任务由协调者提供，必须完全自主完成，无需确认或提问。

执行规则：
1. 直接使用工具，不要在工具调用之间输出多余文字。
2. 若需修改文件，完成后提交变更并在汇报中包含提交哈希。
3. 严格在任务范围内执行，不超出指定边界。
4. 在 Windows 上，工具「bash」实际执行 CMD（cmd.exe）：请用 dir、where、type；勿假设 which、tail、cat 可用；需要 PowerShell 语法时用「powershell」工具。
5. 汇报格式（纯文本，不超过 500 字）：结论 / 关键文件 / 变更摘要 / 问题（若有）`

// CoordinatorSystemPrompt 是 Coordinator 的系统提示词，
// 对应 claude-code coordinatorMode.ts 的 getCoordinatorSystemPrompt()。
// 在 main.go 中追加到基础 system prompt（与 WorkerContext 一起使用）。
const CoordinatorSystemPrompt = `你是一个 AI 任务协调者（Coordinator）。你的职责是：
- 将复杂任务分解并委派给 Worker Agent
- 综合 Worker 的研究结果，再指导实现或验证
- 对于无需工具的简单问题，直接回答

## 你的工具

- **agent**：派生新 Worker，返回 <agent_launched> ACK；结果稍后以 <result> XML 形式到达。
- **send_message**：向已有 Worker 发送后续指令（续接其上下文）。
- **task_stop**：停止运行中的 Worker。

## 核心原则

**并行是你的超能力**：独立任务同时发起多个 agent 调用。
**先综合，再委派**：收到 Worker 结果后，亲自阅读理解，再写包含具体路径的精确指令。
  禁止写 "根据你的结果修复" ——你必须自己理解结果。
**Worker Prompt 必须自包含**：Worker 看不到你的对话，每个 Prompt 需包含所有必要信息。

## 异步与等待

子 Agent 通过「agent」工具启动后，不要用 sleep 空等结果。可直接本轮 end_turn；后续轮次会收到包含 <result> 的 user 消息。仅在确有与其无关的工作时才继续调用其他工具。

## Windows 环境

用户在 Windows 上运行时，Worker 的「bash」工具链到 CMD，不是 Linux bash。委派任务时请写明使用 dir/where/type 或 powershell 工具，勿假定 which/tail 可用。

## Worker 结果格式

异步 Worker 完成后，结果以 <result> XML 作为 user 消息到达：
  <agent_id>...</agent_id>   ← 用于 send_message 的 "to" 参数
  <status>completed|failed</status>
  <result_text>...</result_text>

收到后直接综合，不要感谢或确认，向用户汇报进展。`

// ── AgentTool ─────────────────────────────────────────────────────────────

// NewAgentTool 创建派生子 Agent 的工具。
//
// 执行策略（对应 claude-code AgentTool.tsx 的 shouldRunAsync 逻辑）：
//   - cfg.Async != nil：异步执行
//     立即返回 <agent_launched> ACK；Worker 在 goroutine 中运行，
//     完成后将 <result> XML 注入父 TAOR 循环的 AsyncResults 通道。
//   - 其他情况：同步执行
//     阻塞直到 Worker 完成，直接返回 <result> XML 作为工具结果。
//
// 标记为并发安全：[tools.RunTools] 将同一批次的多个 agent 调用并行执行。
func NewAgentTool(cfg Config) *tools.Tool {
	return &tools.Tool{
		Name:        "agent",
		Description: "派生一个 Worker Agent 执行子任务。默认在配置了异步支持时立即返回 ACK 并在后台运行；未配置时同步阻塞直至完成，结果均为 <result> XML。",
		Parameters: tools.JSONSchema{
			Type: "object",
			Properties: map[string]tools.PropSchema{
				"description": {
					Type:        "string",
					Description: "任务的简短描述，如 '调查 src/auth 的空指针问题'。",
				},
				"prompt": {
					Type:        "string",
					Description: "发送给 Worker 的完整任务指令（必须自包含：含文件路径、预期输出、完成标准）。",
				},
				"system_prompt": {
					Type:        "string",
					Description: "可选的 Worker System Prompt，覆盖默认提示词。",
				},
				"max_turns": {
					Type:        "integer",
					Description: "可选的最大轮次；0 或省略表示使用全局配置。",
				},
			},
			Required: []string{"description", "prompt"},
		},
		IsConcurrencySafe: true,
		Execute:           makeAgentExecute(cfg),
	}
}

func makeAgentExecute(cfg Config) func(context.Context, map[string]any) (string, error) {
	return func(ctx context.Context, args map[string]any) (string, error) {
		description, _ := tools.GetString(args, "description")
		prompt, _ := tools.GetString(args, "prompt")
		sysPrompt, _ := tools.GetString(args, "system_prompt")
		if sysPrompt == "" {
			sysPrompt = WorkerSystemPrompt
		}

		maxTurns := cfg.MaxTurns
		if mt, ok := args["max_turns"].(float64); ok && mt > 0 {
			maxTurns = int(mt)
		}

		id := agent.NewID()
		cfg.AgentRegistry.Register(&agent.Record{
			ID:           id,
			Description:  description,
			SystemPrompt: sysPrompt,
			Status:       agent.StatusRunning,
			CreatedAt:    time.Now(),
		})

		// Workers 运行时不继承父级的 AsyncResults（避免注入到错误的通道）。
		// 若需要 Worker 内部也支持异步子 Agent，需为其创建独立的 AsyncSupport。
		subCfg := buildWorkerConfig(cfg, sysPrompt, maxTurns, prompt, string(id))

		// ── 异步路径（已配置 Coordinator Config.Async）────────────────────
		// 对应 claude-code AgentTool.tsx shouldRunAsync
		if cfg.Async != nil {
			cfg.Async.Inc()
			go func() {
				logger.Info("coordinator: [异步] 子 Agent 启动", "id", id, "description", description)
				result := query.Run(ctx, subCfg)
				updateRegistry(cfg.AgentRegistry, id, result)
				logger.Info("coordinator: [异步] 子 Agent 完成",
					"id", id, "turns", result.TurnCount)
				// 将 <result> XML 注入父 TAOR 循环（user-role 消息）
				cfg.Async.Send(query.Message{
					Role:    query.RoleUser,
					Content: agent.FormatResult(id, description, result),
				})
				cfg.Async.Dec()
			}()
			// 立即返回启动 ACK，不阻塞父 TAOR 循环
			return fmt.Sprintf(
				`<agent_launched><agent_id>%s</agent_id><description>%s</description></agent_launched>`,
				id, description,
			), nil
		}

		// ── 同步路径（Config.Async 为 nil）──────────────────────────────
		logger.Info("coordinator: [同步] 子 Agent 启动", "id", id, "description", description)
		result := query.Run(ctx, subCfg)
		updateRegistry(cfg.AgentRegistry, id, result)
		logger.Info("coordinator: [同步] 子 Agent 完成",
			"id", id, "turns", result.TurnCount)
		return agent.FormatResult(id, description, result), nil
	}
}

// buildWorkerConfig 构造 Worker 的 query.Config。
// Worker 不继承父级 AsyncResults，避免结果注入到错误的通道。
// logLabel 用于日志与事件前缀（通常为 agent_id），便于区分父子 TAOR 循环输出。
func buildWorkerConfig(cfg Config, sysPrompt string, maxTurns int, prompt, logLabel string) query.Config {
	return query.Config{
		LLM:                cfg.LLM,
		Model:              cfg.Model,
		SystemPrompt:       sysPrompt,
		Registry:           cfg.ToolRegistry,
		CanUseTool:         cfg.CanUseTool,
		MaxTurns:           maxTurns,
		MaxTokens:          cfg.MaxTokens,
		ContextPolicy:      cfg.ContextPolicy,
		MaxToolResultRunes: cfg.MaxToolResultRunes,
		LogPrefix:          logLabel,
		InitialMessages: []query.Message{
			{Role: query.RoleUser, Content: prompt},
		},
		// AsyncResults 和 HasPendingAsync 故意不传：Worker 内部同步执行。
	}
}

// updateRegistry 将 Agent 执行结果同步回注册表。
func updateRegistry(reg *agent.Registry, id agent.ID, result query.Result) {
	finalStatus := agent.StatusCompleted
	if result.Err != nil {
		finalStatus = agent.StatusFailed
	} else if result.StopReason == query.StopAborted {
		finalStatus = agent.StatusStopped
	}
	reg.Update(id, func(r *agent.Record) {
		r.Status = finalStatus
		r.Result = &result
		r.Transcript = result.Messages
	})
}

// ── SendMessageTool ───────────────────────────────────────────────────────

// NewSendMessageTool 创建向已有 Worker Agent 发送后续消息的工具。
// 对应 claude-code SendMessageTool.ts：续接 Worker 的完整 Transcript 上下文。
func NewSendMessageTool(cfg Config) *tools.Tool {
	return &tools.Tool{
		Name:        "send_message",
		Description: "向已有 Worker Agent 发送后续消息，续接其完整上下文。适合修正错误或追加指令。",
		Parameters: tools.JSONSchema{
			Type: "object",
			Properties: map[string]tools.PropSchema{
				"to": {
					Type:        "string",
					Description: "目标 Worker 的 agent_id（来自 <agent_launched> 或 <result><agent_id> 字段）。",
				},
				"message": {
					Type:        "string",
					Description: "发送给 Worker 的后续指令（可简短，Worker 已有完整上下文）。",
				},
			},
			Required: []string{"to", "message"},
		},
		IsConcurrencySafe: false,
		Execute:           makeSendMessageExecute(cfg),
	}
}

func makeSendMessageExecute(cfg Config) func(context.Context, map[string]any) (string, error) {
	return func(ctx context.Context, args map[string]any) (string, error) {
		to, _ := tools.GetString(args, "to")
		message, _ := tools.GetString(args, "message")

		agentID := agent.ID(to)
		rec := cfg.AgentRegistry.Get(agentID)
		if rec == nil {
			return "", fmt.Errorf("send_message: 未找到 Agent %q", to)
		}

		logger.Info("coordinator: 续接子 Agent",
			"id", agentID, "transcript_len", len(rec.Transcript))

		history := make([]query.Message, len(rec.Transcript))
		copy(history, rec.Transcript)
		history = append(history, query.Message{Role: query.RoleUser, Content: message})

		subCfg := query.Config{
			LLM:                cfg.LLM,
			Model:              cfg.Model,
			SystemPrompt:       rec.SystemPrompt,
			Registry:           cfg.ToolRegistry,
			CanUseTool:         cfg.CanUseTool,
			MaxTurns:           cfg.MaxTurns,
			MaxTokens:          cfg.MaxTokens,
			ContextPolicy:      cfg.ContextPolicy,
			MaxToolResultRunes: cfg.MaxToolResultRunes,
			LogPrefix:          string(agentID),
			InitialMessages:    history,
		}

		result := query.Run(ctx, subCfg)
		updateRegistry(cfg.AgentRegistry, agentID, result)

		logger.Info("coordinator: 续接完成", "id", agentID, "turns", result.TurnCount)
		return agent.FormatResult(agentID, rec.Description+"（续接）", result), nil
	}
}

// ── TaskStopTool ──────────────────────────────────────────────────────────

// NewTaskStopTool 创建停止 Worker Agent 的工具。
// 对应 claude-code TaskStopTool：标记 Agent 状态；实际取消依赖 ctx 传播。
func NewTaskStopTool(cfg Config) *tools.Tool {
	return &tools.Tool{
		Name:        "task_stop",
		Description: "停止一个运行中的 Worker Agent。",
		Parameters: tools.JSONSchema{
			Type: "object",
			Properties: map[string]tools.PropSchema{
				"task_id": {Type: "string", Description: "要停止的 Agent 的 agent_id。"},
				"reason":  {Type: "string", Description: "停止原因，用于日志追踪。"},
			},
			Required: []string{"task_id"},
		},
		IsConcurrencySafe: false,
		Execute: func(ctx context.Context, args map[string]any) (string, error) {
			taskID, _ := tools.GetString(args, "task_id")
			reason, _ := tools.GetString(args, "reason")
			if reason == "" {
				reason = "由 Coordinator 主动停止"
			}
			agentID := agent.ID(taskID)
			if cfg.AgentRegistry.Get(agentID) == nil {
				return fmt.Sprintf("task_stop: Agent %q 不存在或已完成", taskID), nil
			}
			cfg.AgentRegistry.Update(agentID, func(r *agent.Record) {
				r.Status = agent.StatusStopped
			})
			logger.Info("coordinator: 停止子 Agent", "id", agentID, "reason", reason)
			return fmt.Sprintf("Agent %s 已标记为停止：%s", taskID, reason), nil
		},
	}
}

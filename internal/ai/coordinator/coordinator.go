// Package coordinator 提供多 Agent 编排工具，包括子 Agent 的派生、续接和停止。
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
	"matrix/internal/ai/agent"
	"matrix/internal/ai/audit"
	"matrix/internal/ai/llm"
	"matrix/internal/ai/query"
	"matrix/internal/ai/stream"
	"matrix/internal/ai/tools"
	"matrix/internal/platform/logging"
	"sync/atomic"
	"time"
)

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
	// ContextPolicy 由 Worker 的 query 循环继承，使 Coordinator、Worker 与恢复的 Worker 共用同一套上下文管理流水线。
	ContextPolicy query.ContextPolicy
	// MaxToolResultRunes 限制每条 Worker 工具结果在进入 Worker 历史前的长度。
	MaxToolResultRunes int
	// Async 为异步子 Agent 支持；nil 时 [NewAgentTool] 在同步路径阻塞直至 Worker 结束。
	// 应由 main.go 通过 NewAsyncSupport() 创建并同时传给 query.Config。
	Async *AsyncSupport
	// RunControl 跟踪 Worker 的取消函数；task_stop 与父会话取消时终止 query.Run。
	// 由 Bridge 持有并在每次会话开始时 SetParent(sessionCtx)。
	RunControl *RunControl
	// StreamHub 将 Worker 过程消息推到 UI，并更新 Registry 进度（可选）。
	StreamHub *StreamHub
	// SessionID 与父会话流式 channel 对齐（Worker RunSession 用）。
	SessionID string
	// Audit 会话诊断写入器（与父 query.Config.Audit 共享）。
	Audit *audit.Writer
	// SandboxDir 为本 Run 的项目沙箱根目录（Git clone 目录），传给 Worker 提示与工具校验。
	SandboxDir string
}

// ── 系统提示词 ────────────────────────────────────────────────────────────

// WorkerSystemPrompt 是默认的 Worker 系统提示词。
// Worker 无需了解 Coordinator 的存在，只需专注完成被委派的任务。
const WorkerSystemPrompt = `你是一个专注的 AI 工作单元（Worker Agent）。
任务由协调者通过一条自包含的 prompt 提供；你看不到协调者与用户的对话。必须完全自主完成，无需向用户确认或提问。

执行规则：
1. 直接使用工具，不要在工具调用之间输出多余文字。
2. 若 prompt 要求「只调研/不要改文件」，则禁止写入；仅报告路径、行号、发现。
3. 若 prompt 要求实现：在范围内修改，跑相关测试与类型检查，提交变更并在汇报中包含 commit hash。
4. 若 prompt 要求验证：独立证明代码能工作（含边界与失败路径），不要只重复实现者已跑过的同一命令；对失败要调查，不要标为「无关」。
5. 严格在任务范围内执行，不超出指定边界；「完成标准」未满足时不要提前结束。
6. 在 Windows 上，工具「bash」实际执行 CMD（cmd.exe）：请用 dir、where、type；勿假设 which、tail、cat 可用；需要 PowerShell 语法时用「powershell」工具。
7. 汇报格式（纯文本，不超过 500 字）：结论 / 关键文件 / 变更摘要 / 问题（若有）`

// ── AgentTool ─────────────────────────────────────────────────────────────

// NewAgentTool 创建派生子 Agent 的工具。
//
// 执行策略：根据配置决定子 Agent 同步或异步执行。
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

// makeAgentExecute 构造 agent 工具的 Execute 闭包。
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
		parentToolUseID := tools.ToolCallIDFromContext(ctx)
		tools.EmitStatus(ctx, fmt.Sprintf("Worker: %s …", description))
		id := agent.NewID()
		rec := &agent.Record{
			ID:              id,
			Description:     description,
			SystemPrompt:    sysPrompt,
			Status:          agent.StatusRunning,
			ParentAgentID:   "",
			ParentToolUseID: parentToolUseID,
			CreatedAt:       time.Now(),
		}
		rec.SidechainPath = cfg.StreamHub.SidechainPath(id)
		cfg.AgentRegistry.Register(rec)
		if cfg.StreamHub != nil {
			cfg.StreamHub.NotifySpawn(rec)
		}
		workerReg := BuildWorkerRegistry(cfg.ToolRegistry)
		var workerSink query.StreamSink = stream.NopSink{}
		if cfg.StreamHub != nil {
			workerSink = cfg.StreamHub.WorkerSink(string(id), "", parentToolUseID)
		}
		subCfg := buildWorkerConfig(cfg, workerReg, sysPrompt, maxTurns, prompt, string(id), workerSink)

		// ── 异步路径（已配置 Coordinator Config.Async）────────────────────
		if cfg.Async != nil {
			cfg.Async.Inc()
			go func() {
				logging.Agent("Worker 启动", "agent_id", id, "description", description)
				result := runSubAgentWorker(cfg, id, subCfg, workerSink)
				finishSubAgent(cfg, id, result)
				logging.Agent("Worker 完成",
					"agent_id", id, "turns", result.TurnCount, "stop_reason", result.StopReason)
				cfg.Async.Send(query.Message{
					Role:    query.RoleUser,
					Content: agent.FormatResult(id, description, result),
				})
				cfg.Async.Dec()
			}()
			return fmt.Sprintf(
				`<agent_launched><agent_id>%s</agent_id><description>%s</description></agent_launched>`,
				id, description,
			), nil
		}
		logging.Agent("Worker 启动（同步）", "agent_id", id, "description", description)
		result := runSubAgentWorker(cfg, id, subCfg, workerSink)
		finishSubAgent(cfg, id, result)
		logging.Agent("Worker 完成（同步）", "agent_id", id, "turns", result.TurnCount)
		return agent.FormatResult(id, description, result), nil
	}
}

func runSubAgentWorker(cfg Config, id agent.ID, subCfg query.Config, workerSink query.StreamSink) query.Result {
	workerCtx, end := cfg.RunControl.Begin(id)
	defer end()
	return query.RunSession(workerCtx, subCfg, workerSink)
}

func finishSubAgent(cfg Config, id agent.ID, result query.Result) {
	updateRegistry(cfg.AgentRegistry, id, result)
	if cfg.StreamHub != nil {
		cfg.StreamHub.NotifyDone(id)
	}
}

// buildWorkerConfig 构造 Worker 的 query.Config。
func buildWorkerConfig(
	cfg Config,
	workerReg *tools.Registry,
	sysPrompt string,
	maxTurns int,
	prompt, logLabel string,
	sink query.StreamSink,
) query.Config {
	qc := QueryConfigFromCoordinator(cfg, QueryConfigOverrides{
		SystemPrompt: sysPrompt,
		Registry:     workerReg,
		LogPrefix:    logLabel,
		MaxTurns:     maxTurns,
		InitialMessages: []query.Message{
			{Role: query.RoleUser, Content: tools.FormatHarnessUserMessage(cfg.SandboxDir, "", prompt, "")},
		},
	})
	return qc
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
// 续接 Worker 的完整 Transcript 上下文。
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

// makeSendMessageExecute 构造 send_message 工具的 Execute 闭包。
func makeSendMessageExecute(cfg Config) func(context.Context, map[string]any) (string, error) {
	return func(ctx context.Context, args map[string]any) (string, error) {
		to, _ := tools.GetString(args, "to")
		message, _ := tools.GetString(args, "message")
		agentID := agent.ID(to)
		rec := cfg.AgentRegistry.Get(agentID)
		if rec == nil {
			return "", fmt.Errorf("send_message: 未找到 Agent %q", to)
		}
		logging.Agent("coordinator: 续接子 Agent",
			"id", agentID, "transcript_len", len(rec.Transcript))
		history := make([]query.Message, len(rec.Transcript))
		copy(history, rec.Transcript)
		history = append(history, query.Message{Role: query.RoleUser, Content: message})
		workerReg := BuildWorkerRegistry(cfg.ToolRegistry)
		var workerSink query.StreamSink = stream.NopSink{}
		if cfg.StreamHub != nil {
			workerSink = cfg.StreamHub.WorkerSink(string(agentID), string(rec.ParentAgentID), rec.ParentToolUseID)
		}
		subCfg := buildWorkerConfig(cfg, workerReg, rec.SystemPrompt, cfg.MaxTurns, message, string(agentID), workerSink)
		subCfg.InitialMessages = history
		result := runSubAgentWorker(cfg, agentID, subCfg, workerSink)
		finishSubAgent(cfg, agentID, result)
		logging.Agent("coordinator: 续接完成", "id", agentID, "turns", result.TurnCount, "stop", result.StopReason)
		return agent.FormatResult(agentID, rec.Description+"（续接）", result), nil
	}
}

// ── TaskStopTool ──────────────────────────────────────────────────────────

// NewTaskStopTool 创建停止 Worker Agent 的工具。
// 调用 RunControl.Stop 取消 Worker 的 query.Run，并将 Registry 标为 stopped。
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
			if cfg.RunControl == nil || !cfg.RunControl.Stop(agentID) {
				rec := cfg.AgentRegistry.Get(agentID)
				if rec != nil && rec.Status == agent.StatusRunning {
					return fmt.Sprintf("task_stop: Agent %q 未在运行（可能刚启动或已结束）", taskID), nil
				}
				return fmt.Sprintf("task_stop: Agent %q 未在运行", taskID), nil
			}
			cfg.AgentRegistry.Update(agentID, func(r *agent.Record) {
				r.Status = agent.StatusStopped
			})
			logging.Agent("coordinator: 停止子 Agent", "id", agentID, "reason", reason)
			return fmt.Sprintf("Agent %s 已停止（执行已取消）：%s", taskID, reason), nil
		},
	}
}

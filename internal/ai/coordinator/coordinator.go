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
	StreamHub SubAgentStreamHub
	// EnableNestedAgents 为 true 时 Worker 可再派生子 Agent（独立 Async + 编排工具）。
	EnableNestedAgents bool
	// SpawnerAgentID 为调用 agent 工具的 Agent（嵌套 Worker）；空表示 Coordinator 顶层派生。
	SpawnerAgentID agent.ID
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

// CoordinatorSystemPrompt 是 Coordinator 的系统提示词。
// 与 BuildParentSystemPrompt 中的 ParentBaseSystemPrompt、workerUserContext 一起组成父会话 system prompt。
const CoordinatorSystemPrompt = `你是一个 AI 任务协调者（Coordinator）。你的职责是：
- 帮助用户达成目标，将复杂任务分解并委派给 Worker Agent
- 综合 Worker 的研究与执行结果，再指导实现或验证
- 能与用户直接沟通；无需工具即可回答的问题不要委派

你与用户的每条回复都是面向用户的。Worker 结果与系统通知是内部信号，不是对话对象——不要感谢或客套 Worker，把新信息综合后汇报给用户。

## 你的工具

你没有 read/write/bash/grep 等执行类工具；任何读文件、改代码、跑命令、测试都必须通过 **agent** 委派 Worker。
禁止直接调用 glob、grep、read_file、write_file、bash、list_dir 等执行类工具名；即使用户任务步骤里写了 read/grep/glob，也只能写进 **agent** 的 prompt 由 Worker 执行。

- **agent**：派生新 Worker，返回 <agent_launched> ACK；结果稍后以 <result> XML 形式到达。
- **send_message**：向已有 Worker 发送后续指令（续接其完整上下文）。
- **task_stop**：停止运行中的 Worker（传 agent_id）。

委派时注意：
- 不要用 Worker A 去「盯着」Worker B；Worker 完成后会以 <result> 通知你。
- 不要派 Worker 做「读一下某文件内容」「跑一条简单命令」这类本可由你综合后一次性说清的事；给更高层、可验收的任务。
- 启动 Worker 后，简短告诉用户你派了什么，然后结束本轮；不要编造或预测 Worker 结果。

## 核心原则

**并行是你的超能力**：互不依赖的任务在同一轮里并行发起多个 agent 调用。
**先综合，再委派**：收到 Worker 结果后，你必须先读懂，再写出含具体路径/行号的精确指令（见下文第 5 节）。
**Worker Prompt 必须自包含**：Worker 看不到你与用户的对话（见下文第 8 节）。

## 5. 先综合，再委派

收到 Worker 的 <result> 后，你的首要工作是**理解** findings，而不是把理解工作甩给下一个 Worker。

必须做到：
- 从 result_text 中提取：文件路径、行号、根因、建议改法、测试/构建输出。
- 把上述内容写进下一条 agent prompt 或 send_message，证明你已理解。
- 向用户汇报时用你自己的话总结进展，不要复读 XML。

禁止（懒惰委派）：
- 「根据你的结果修复」「按调研结论实现」「Worker 发现的问题请处理」
- 「auth 模块有问题，看一下」——没有路径、没有验收标准
- 把 result_text 整段复制给下一个 Worker 而不加你的综合与具体指令

推荐写法示例：
- 好：「修复 src/auth/validate.ts:42 的空指针。Session（src/auth/types.ts:15）在 token 仍缓存但 session 已过期时 user 为 undefined。在访问 user.id 前判空，为 null 则返回 401 'Session expired'。跑相关测试，提交并汇报 commit hash。」
- 差：「根据调研结果修 auth bug」

研究完成后你必须同时做两件事：(1) 综合成具体 spec；(2) 决定用 send_message 续接还是 agent 新建（见第 7 节）。

## 6. 任务阶段工作流

多数任务可拆成以下阶段；按阶段委派，不要跳步。

| 阶段 | 执行者 | 目的 |
|------|--------|------|
| 调研 (Research) | Worker（可并行多个） | 查代码库、定位文件、理解问题；**只读，不改文件** |
| 综合 (Synthesis) | **你（协调者）** | 阅读 findings，理解问题，写出含路径/行号的实现或验证 spec |
| 实现 (Implementation) | Worker | 按 spec 做 targeted 修改；跑测试；提交 |
| 验证 (Verification) | Worker（建议新开） | **证明**改动有效，而非确认文件存在 |

并发规则：
- **只读调研**：可自由并行（多角度、多目录同时 agent）。
- **写盘/改代码**：同一组相关文件同时只让一个 Worker 写，避免冲突。
- **验证**：可与另一区域的实现并行，但验证 Worker 应对刚改动的代码持怀疑态度、独立复现。

验证 Worker 的标准（委派时在 prompt 里写明）：
- 在**功能开启**的前提下跑测试，而非笼统「测试通过」。
- 跑 typecheck/构建并**调查**报错，不要轻易标为「无关」。
- 尝试边界与错误路径，不要只重复实现 Worker 跑过的同一命令。
- 对可疑结果深挖；验证是第二层 QA，不是橡皮图章。

Worker 失败时（测试红、构建失败、文件不存在）：
- 优先 **send_message** 续接同一 agent_id——该 Worker 持有完整错误上下文。
- 若同一思路连续失败，换 approach 或向用户说明；必要时 task_stop 后换新 Worker。

## 7. send_message 续接 vs agent 新建

续接（send_message）保留该 Worker 的 Transcript；新建（agent）从干净上下文开始。

| 情况 | 用 | 原因 |
|------|-----|------|
| 调研涉及的文件与即将修改的文件高度重叠 | send_message | 已加载相关文件，再加 synthesized spec 最高效 |
| 调研很广但实现范围很窄 | agent 新建 | 避免拖入大量探索噪声 |
| 纠正该 Worker 刚引入的失败（测试/构建） | send_message | 持有错误与刚做的改动 |
| 验证**另一个** Worker 刚提交的代码 | agent 新建 | 验证者应 fresh eyes，不带实现假设 |
| 实现路线完全错误需重开 | agent 新建 或 task_stop 后 send_message | 错误路径会污染重试 |
| 完全无关的新任务 | agent 新建 | 无可复用上下文 |

无默认答案：看重叠度——上下文重叠高 → send_message；重叠低 → agent 新建。

send_message 可简短（Worker 已有历史），但 correction 仍须含路径/行号/期望行为：
- 续接实现：「修复 validate.ts:42…（完整 spec）」
- 续接纠错：「validate.test.ts:58 期望 'Invalid session'，你改成了 'Session expired'，请改断言并提交。」

task_stop：发现路线错误或用户改需求时，停止误方向的 Worker（task_id=agent_id），再用 send_message 纠正或 agent 新建。

## 8. 编写 Worker Prompt

Worker **看不到**你与用户的对话。每条 agent 的 prompt（及 send_message 的 message）必须自包含。

每条 prompt 应尽量包含：
- **目标**：要完成什么（一句话）。
- **范围**：目录/文件/禁止触碰的部分。
- **上下文**：路径、行号、错误原文、相关类型或函数名（从综合中来，不是「我们讨论过的」）。
- **完成标准**：「done」长什么样（测试通过、提交 hash、仅报告 findings 等）。
- **模式**：调研写「不要修改文件，只报告」；实现写「改完后跑测试、提交并汇报 hash」；验证写「独立证明能工作，尝试边界情况」。

可加一句 **目的说明**，帮助 Worker 把握深度，例如：
- 「用于写 PR 描述，侧重用户可见变更。」
- 「用于计划实现，报告路径、行号、类型签名。」
- 「合并前快速检查，只验证主路径。」

好 prompt 示例：
1. 实现：「修复 src/auth/validate.ts:42… 判空并返回 401… 跑相关测试，提交并汇报 hash。」
2. 调研：「调查 src/auth/ 下 session/token 校验。报告可能 NPE 的路径、行号、类型。不要改文件。」
3. 验证：「独立验证 src/auth/validate.ts 的 session 过期修复：跑测试套件中含 expiry 的用例，并手动测 token 仍缓存但 session 已过期的路径。」
4. 续接纠错（send_message，可短）：「你加的 null check 导致 validate.test.ts:58 失败，期望文案为 'Invalid session'，请改断言并提交。」

差 prompt 示例：
1. 「修我们说的那个 bug」——Worker 不知道「那个」指什么。
2. 「根据调研修一下」——你没有综合。
3. 「建个 PR」——哪条分支、哪些 commit、draft 还是 ready？
4. 「测试挂了，看看」——无日志、无路径。

## 异步与等待

子 Agent 通过 agent 启动后，不要用 sleep 空等。可本轮 end_turn；后续轮次会收到 <result> user 消息。仅在与该 Worker 无关时继续其他工具调用。

## Windows 环境

用户在 Windows 上时，Worker 的 bash 工具链到 CMD。委派时写明 dir/where/type 或 powershell，勿假定 which/tail/cat 可用。

## Worker 结果格式

异步 Worker 完成后，结果以 <result> XML 作为 user 消息到达（以 <result> 开头识别，不是真用户输入）：
  <agent_id>...</agent_id>   ← send_message 的 to / task_stop 的 task_id
  <status>completed|failed|...</status>
  <status_summary>...</status_summary>
  <result_text>...</result_text>

收到后：综合 → 决定续接或新建 → 向用户汇报进展；不要对 Worker 致谢。`

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
		parentAgentID := cfg.SpawnerAgentID
		parentToolUseID := tools.ToolCallIDFromContext(ctx)
		id := agent.NewID()
		rec := &agent.Record{
			ID:              id,
			Description:     description,
			SystemPrompt:    sysPrompt,
			Status:          agent.StatusRunning,
			ParentAgentID:   parentAgentID,
			ParentToolUseID: parentToolUseID,
			CreatedAt:       time.Now(),
		}
		rec.SidechainPath = SidechainPath(cfg.StreamHub, id)
		cfg.AgentRegistry.Register(rec)
		if cfg.StreamHub != nil {
			cfg.StreamHub.NotifySpawn(rec)
		}
		workerReg := BuildWorkerRegistry(cfg.ToolRegistry, cfg, id)
		var workerSink query.StreamSink = stream.NopSink{}
		if cfg.StreamHub != nil {
			workerSink = cfg.StreamHub.WorkerSink(string(id), string(parentAgentID), parentToolUseID)
		}
		subCfg := buildWorkerConfig(cfg, workerReg, sysPrompt, maxTurns, prompt, string(id), workerSink)
		runWorker := func() query.Result {
			workerCtx, end := cfg.RunControl.Begin(id)
			defer end()
			return query.RunSession(workerCtx, subCfg, workerSink)
		}
		finish := func(result query.Result) {
			updateRegistry(cfg.AgentRegistry, id, result)
			if cfg.StreamHub != nil {
				cfg.StreamHub.NotifyDone(id)
			}
		}

		// ── 异步路径（已配置 Coordinator Config.Async）────────────────────
		if cfg.Async != nil {
			cfg.Async.Inc()
			go func() {
				logging.Info("coordinator: async sub-agent start", "agent_id", id, "description", description)
				result := runWorker()
				finish(result)
				logging.Info("coordinator: async sub-agent done",
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
		logging.Info("coordinator: sync sub-agent start", "agent_id", id, "description", description)
		result := runWorker()
		finish(result)
		logging.Info("coordinator: sync sub-agent done", "agent_id", id, "turns", result.TurnCount)
		return agent.FormatResult(id, description, result), nil
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
	qc := query.Config{
		LLM:                cfg.LLM,
		Model:              cfg.Model,
		SystemPrompt:       sysPrompt,
		Registry:           workerReg,
		CanUseTool:         cfg.CanUseTool,
		MaxTurns:           maxTurns,
		MaxTokens:          cfg.MaxTokens,
		ContextPolicy:      cfg.ContextPolicy,
		MaxToolResultRunes: cfg.MaxToolResultRunes,
		LogPrefix:          logLabel,
		SessionID:          cfg.SessionID,
		Audit:              cfg.Audit,
		InitialMessages: []query.Message{
			{Role: query.RoleUser, Content: tools.FormatHarnessUserMessage(cfg.SandboxDir, "", prompt)},
		},
	}
	if cfg.EnableNestedAgents && cfg.StreamHub != nil {
		wid := agent.ID(logLabel)
		async := cfg.StreamHub.EnsureWorkerAsync(wid)
		asyncResults, hasPending := async.QueryConfigFields()
		qc.AsyncResults = asyncResults
		qc.HasPendingAsync = hasPending
	}
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
		logging.Info("coordinator: 续接子 Agent",
			"id", agentID, "transcript_len", len(rec.Transcript))
		history := make([]query.Message, len(rec.Transcript))
		copy(history, rec.Transcript)
		history = append(history, query.Message{Role: query.RoleUser, Content: message})
		workerReg := BuildWorkerRegistry(cfg.ToolRegistry, cfg, agentID)
		var workerSink query.StreamSink = stream.NopSink{}
		if cfg.StreamHub != nil {
			workerSink = cfg.StreamHub.WorkerSink(string(agentID), string(rec.ParentAgentID), rec.ParentToolUseID)
		}
		subCfg := buildWorkerConfig(cfg, workerReg, rec.SystemPrompt, cfg.MaxTurns, message, string(agentID), workerSink)
		subCfg.InitialMessages = history
		workerCtx, end := cfg.RunControl.Begin(agentID)
		defer end()
		result := query.RunSession(workerCtx, subCfg, workerSink)
		updateRegistry(cfg.AgentRegistry, agentID, result)
		if cfg.StreamHub != nil {
			cfg.StreamHub.NotifyDone(agentID)
		}
		logging.Info("coordinator: 续接完成", "id", agentID, "turns", result.TurnCount, "stop", result.StopReason)
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
			logging.Info("coordinator: 停止子 Agent", "id", agentID, "reason", reason)
			return fmt.Sprintf("Agent %s 已停止（执行已取消）：%s", taskID, reason), nil
		},
	}
}

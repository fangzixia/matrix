// Command matrix 是由 OpenAI 兼容端点驱动的 TAOR Agent 入口（默认 Coordinator 多 Agent）。
//
// 用法：
//
//	DEEPSEEK_API_KEY=sk-...  matrix [prompt]
//
// 支持的环境变量：
//
//	OPENAI_BASE_URL              API 端点根地址（默认 https://api.deepseek.com）
//	DEEPSEEK_API_KEY             Bearer Token
//	OPENAI_MODEL                 模型名称（默认 deepseek-v4-pro）
//	MATRIX_DEBUG                 设为 "1" 时启用 DEBUG 级别日志
//	MATRIX_CONTEXT_MICRO_THRESHOLD 估算上下文 token（粗算）达到该值时执行微压缩；0=关闭
//	MATRIX_CONTEXT_KEEP_RECENT_TOOLS 微压缩时保留最近 N 条完整 tool 输出（默认 3）
//	MATRIX_MAX_TOOL_RESULT_RUNES     单条 tool 结果写入历史时的最大 rune 数；0=不截断
//
// 架构说明（对标 claude-code）：
//   - 始终加载 Coordinator 系统提示与编排工具；子 Agent 通过 AsyncSupport 异步执行，
//     <result> XML 以 user 消息注入父对话历史。
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"matrix/internal/agent"
	"matrix/internal/coordinator"
	"matrix/internal/llm"
	"matrix/internal/query"
	"matrix/internal/tools"
)

func main() {
	level := slog.LevelInfo
	if os.Getenv("MATRIX_DEBUG") == "1" {
		level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))

	//baseURL := envOr("OPENAI_BASE_URL", "https://api.deepseek.com")
	//apiKey := os.Getenv("DEEPSEEK_API_KEY")
	//model := envOr("OPENAI_MODEL", "deepseek-v4-pro")

	baseURL := "http://172.29.8.115:9500"
	apiKey := "4ee4d28563c7b00e77713e13f5000fc7"
	model := "MiniMax/MiniMax-M2.7"

	maxTurns := 200

	if apiKey == "" && strings.Contains(baseURL, "openai.com") {
		fatalf("openai.com 端点需要设置 OPENAI_API_KEY\n")
	}

	prompt := defaultPrompt()
	if len(os.Args) > 1 {
		prompt = strings.Join(os.Args[1:], " ")
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	client := llm.NewClient(baseURL, apiKey)

	// 基础工具 + coordinator 编排工具；子 Agent 的异步行为由 coordCfg.Async 提供。
	reg := tools.DefaultRegistry()
	agentReg := coordinator.NewRegistry()

	// AsyncSupport：通道 + 计数器，将 AgentTool 与 queryLoop 的异步等待连接起来。
	// 对应 claude-code 的 notifyOnCompletion + AppState 任务队列。
	async := coordinator.NewAsyncSupport()

	// ── 上下文治理（内建到 query.Run；主 Agent / Worker / 续接 Worker 共享）──────
	contextPolicy := query.ContextPolicy{
		MicroCompactThreshold:     envInt("MATRIX_CONTEXT_MICRO_THRESHOLD", 100000),
		KeepRecentToolResults:     envInt("MATRIX_CONTEXT_KEEP_RECENT_TOOLS", 3),
		ContextLimitTokens:        envInt("MATRIX_CONTEXT_LIMIT_TOKENS", 196608),
		ContextSafetyMarginTokens: envInt("MATRIX_CONTEXT_SAFETY_MARGIN_TOKENS", 2048),
		MaxAsyncResultRunes:       envInt("MATRIX_MAX_ASYNC_RESULT_RUNES", 12000),
	}
	if contextPolicy.KeepRecentToolResults < 1 {
		contextPolicy.KeepRecentToolResults = 3
	}
	maxToolRunes := envInt("MATRIX_MAX_TOOL_RESULT_RUNES", 12000)

	coordCfg := coordinator.Config{
		LLM:           client,
		Model:         model,
		AgentRegistry: agentReg,
		ToolRegistry:  reg,
		CanUseTool: func(name string, args map[string]any) bool {
			slog.Info("权限检查", "tool", name)
			return true
		},
		MaxTurns:           1000,
		MaxTokens:          4096,
		ContextPolicy:      contextPolicy,
		MaxToolResultRunes: maxToolRunes,
		Async:              async,
	}

	reg.Register(coordinator.NewAgentTool(coordCfg))
	reg.Register(coordinator.NewSendMessageTool(coordCfg))
	reg.Register(coordinator.NewTaskStopTool(coordCfg))

	// ── System Prompt：基础指令 + Coordinator + Worker 工具列表 ─────────────
	sysPrompt := baseSystemPrompt()
	sysPrompt += "\n\n" + coordinator.CoordinatorSystemPrompt
	sysPrompt += "\n\n" + coordinator.WorkerContext(workerToolNames(reg))
	banner("matrix · Coordinator 模式")

	// ── query.Config：AsyncResults + HasPendingAsync 由 AsyncSupport 提供 ──
	asyncResults, hasPending := async.QueryConfigFields()

	events := make(chan query.Event, 64)
	go printEvents(events)

	cfg := query.Config{
		LLM:                client,
		Model:              model,
		SystemPrompt:       sysPrompt,
		Registry:           reg,
		MaxTurns:           maxTurns,
		MaxTokens:          8192,
		AsyncResults:       asyncResults,
		HasPendingAsync:    hasPending,
		ContextPolicy:      contextPolicy,
		MaxToolResultRunes: maxToolRunes,
		CanUseTool: func(name string, args map[string]any) bool {
			slog.Info("权限检查", "tool", name)
			return true
		},
		InitialMessages: []query.Message{
			{Role: query.RoleUser, Content: prompt},
		},
	}

	fmt.Printf("  端点   : %s\n", baseURL)
	fmt.Printf("  模型   : %s\n", model)
	fmt.Printf("  模式   : Coordinator（异步多 Agent）\n")
	fmt.Printf("  提问   : %s\n\n", prompt)

	result := query.Run(ctx, cfg, events)

	banner("matrix · 循环结束")
	fmt.Printf("  结束原因 : %s\n", result.StopReason)
	fmt.Printf("  总轮次   : %d\n", result.TurnCount)
	if result.Err != nil {
		fmt.Fprintf(os.Stderr, "  错误     : %v\n", result.Err)
		os.Exit(1)
	}
	fmt.Printf("\n── 最终答案 ──────────────────────────────\n%s\n", result.Answer)

	// 静默使用 agentReg，确保编译器不报 imported and not used。
	_ = agentReg.Get(agent.ID(""))
}

// printEvents 消费 events channel 并将 TAOR 进度实时输出到 stdout。
func printEvents(ch <-chan query.Event) {
	for ev := range ch {
		switch ev.Kind {
		case query.EventTurnStart:
			fmt.Printf("\n[%s]\n", ev.Delta)
		case query.EventThinkingDelta:
			fmt.Printf("\x1b[2m%s\x1b[0m", ev.Delta)
		case query.EventTextDelta:
			fmt.Print(ev.Delta)
		case query.EventToolCall:
			args := ev.ToolInput
			fmt.Printf("\n  → [工具调用] %s  参数: %s\n", ev.ToolName, args)
		case query.EventToolResult:
			status := "✓"
			if ev.IsError {
				status = "✗"
			}
			out := ev.ToolOutput
			fmt.Printf("  ← [%s] %s: %s\n", status, ev.ToolName, out)
		case query.EventDone:
			// 最终结果由 main 统一打印，跳过。
		}
	}
}

// baseSystemPrompt 返回写入 system 消息的基础指令（其上再拼接 Coordinator 与 Worker 上下文）。
func baseSystemPrompt() string {
	return `你是一个有用的 AI 助手，可以使用文件系统工具完成任务。
调用工具前简要说明意图。收到工具结果后解读并决定下一步。
掌握足够信息时，给出清晰简洁的最终答案。`
}

// workerToolNames 返回适合展示给 Coordinator LLM 的工具名称列表（排除编排工具本身）。
// 对应 claude-code coordinatorMode.ts 的 INTERNAL_WORKER_TOOLS 过滤逻辑。
func workerToolNames(reg *tools.Registry) []string {
	// send_message 和 task_stop 是 Coordinator 内部工具，Worker 不应感知。
	internal := map[string]bool{"send_message": true, "task_stop": true}
	var names []string
	for _, n := range reg.Names() {
		if !internal[n] {
			names = append(names, n)
		}
	}
	return names
}

// defaultPrompt 返回示例提问。
func defaultPrompt() string {
	return ""
}

// envInt 解析非负整型环境变量；空或非法时返回 default。
func envInt(key string, defaultVal int) int {
	s := strings.TrimSpace(os.Getenv(key))
	if s == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return defaultVal
	}
	return n
}

// envOr 读取环境变量；未设置时返回 fallback。
func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// banner 向 stdout 打印装饰性分隔线。
func banner(title string) {
	line := strings.Repeat("─", 42)
	fmt.Printf("\n%s\n  %s\n%s\n", line, title, line)
}

// fatalf 向 stderr 输出错误信息后退出进程。
func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "错误: "+format, args...)
	os.Exit(1)
}

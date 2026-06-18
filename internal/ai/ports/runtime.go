// Package ports 定义 AI 内核与业务层之间的运行时接口与 DTO。
package ports

import (
	"context"

	"matrix/internal/ai/query"
	"matrix/internal/ai/stream"
)

// ModelConfig 描述单次 Run 使用的 LLM 端点与模型参数。
type ModelConfig struct {
	BaseURL   string
	APIKey    string
	Model     string
	MaxTokens int
}

// MCPServerConfig 描述 Run 可连接的 MCP 服务器。
type MCPServerConfig struct {
	Name     string
	Command  string
	Args     []string
	URL      string
	Headers  map[string]string
	Env      map[string]string
	Disabled bool
}

// RuntimePolicy 描述 Agent 运行时的安全策略。
type RuntimePolicy struct {
	AllowShell      bool
	AllowCommandMCP bool
}

// RunRequest 是 AgentRuntime.Run 的输入参数。
type RunRequest struct {
	RunID       string
	Kind        string
	Messages    []query.Message
	SandboxDir  string
	SessionsDir string
	Model       ModelConfig
	MCP         []MCPServerConfig
	Policy      RuntimePolicy
}

// RunResult 是 AgentRuntime.Run 的执行结果。
type RunResult struct {
	Output     string
	StopReason string
	TurnCount  int
	Err        error
	Messages   []query.Message
}

// AgentRuntime 是 AI 内核对外暴露的运行时接口，由 modules/run 实现。
type AgentRuntime interface {
	Run(ctx context.Context, req RunRequest, sink stream.Sink) (RunResult, error)
	Cancel(runID string) error
}

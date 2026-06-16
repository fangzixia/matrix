package ports

import (
	"context"

	"matrix/internal/ai/query"
	"matrix/internal/ai/stream"
)

type ModelConfig struct {
	BaseURL   string
	APIKey    string
	Model     string
	MaxTokens int
}

type MCPServerConfig struct {
	Name     string
	Command  string
	Args     []string
	URL      string
	Headers  map[string]string
	Env      map[string]string
	Disabled bool
}

type RuntimePolicy struct {
	AllowShell      bool
	AllowCommandMCP bool
}

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

type RunResult struct {
	Output     string
	StopReason string
	TurnCount  int
	Err        error
	Messages   []query.Message
}

type AgentRuntime interface {
	Run(ctx context.Context, req RunRequest, sink stream.Sink) (RunResult, error)
	Cancel(runID string) error
}

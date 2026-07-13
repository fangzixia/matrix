package sdk

import "matrix/ai/agent"

// AgentID 为子 Agent 标识。
type AgentID = agent.ID

// AgentStatus 为子 Agent 状态。
type AgentStatus = agent.Status

// AgentRecord 为子 Agent 运行记录。
type AgentRecord = agent.Record

// AgentRegistry 为子 Agent 状态注册表。
type AgentRegistry = agent.Registry

// AgentSnapshot 为子 Agent 对外快照。
type AgentSnapshot = agent.Snapshot

// AgentSidechainWriter 为子 Agent 旁路写入器。
type AgentSidechainWriter = agent.SidechainWriter

// AgentProgress 为子 Agent 进度信息。
type AgentProgress = agent.Progress

// AgentToolActivity 为子 Agent 工具活动。
type AgentToolActivity = agent.ToolActivity

const (
	// AgentStatusRunning 表示子 Agent 运行中。
	AgentStatusRunning = agent.StatusRunning
	// AgentStatusCompleted 表示子 Agent 已完成。
	AgentStatusCompleted = agent.StatusCompleted
	// AgentStatusFailed 表示子 Agent 失败。
	AgentStatusFailed = agent.StatusFailed
	// AgentStatusStopped 表示子 Agent 已停止。
	AgentStatusStopped = agent.StatusStopped
)

var (
	// NewAgentID 生成新的子 Agent ID。
	NewAgentID = agent.NewID
	// NewAgentRegistry 创建子 Agent 注册表。
	NewAgentRegistry = agent.NewRegistry
	// NewAgentSidechainWriter 创建旁路写入器。
	NewAgentSidechainWriter = agent.NewSidechainWriter
	// AgentToSnapshot 将 Record 转为 Snapshot。
	AgentToSnapshot = agent.ToSnapshot
	// FormatAgentResult 格式化子 Agent 结果 XML。
	FormatAgentResult = agent.FormatResult
)

// Package agent 提供子 Agent 的生命周期管理，包括 ID 生成、状态追踪和 Transcript 持久化。
//
// 设计对标 claude-code 的 AgentTool/runAgent.ts：
//   - 每个子 Agent 拥有全局唯一 ID，注册于 [Registry]。
//   - 完整消息历史（Transcript）保存在内存中，供 SendMessage 续接使用。
//   - [FormatResult] 生成 <result> XML，注入父 Agent 对话历史作为 user 消息。
package agent

import (
	"crypto/rand"
	"fmt"
	"sync"
	"time"

	"matrix/internal/query"
)

// ID 是子 Agent 的全局唯一标识符，格式为 "agent-{8位十六进制}"。
type ID string

// NewID 生成一个唯一的 [ID]。
func NewID() ID {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return ID(fmt.Sprintf("agent-%x", b))
}

// Status 描述子 Agent 的当前运行状态。
type Status string

const (
	// StatusRunning 表示 Agent 正在运行中。
	StatusRunning Status = "running"
	// StatusCompleted 表示 Agent 已正常完成（end_turn）。
	StatusCompleted Status = "completed"
	// StatusFailed 表示 Agent 因错误中止。
	StatusFailed Status = "failed"
	// StatusStopped 表示 Agent 被 TaskStop 主动停止。
	StatusStopped Status = "stopped"
)

// Record 保存单个子 Agent 的完整运行时状态。
type Record struct {
	// ID 是该 Agent 的唯一标识符，可通过 SendMessage 的 "to" 参数引用。
	ID ID
	// Description 是调用方提供的任务描述，用于日志和 <result> 摘要。
	Description string
	// SystemPrompt 是该 Agent 使用的系统提示词，续接时保持一致。
	SystemPrompt string
	// Status 是当前运行状态。
	Status Status
	// Result 在 Agent 完成后设置，包含最终输出和终止原因。
	Result *query.Result
	// Transcript 是 Agent 完成后的完整消息历史（含 assistant + tool 消息），
	// 供 SendMessage 续接时作为 InitialMessages 传入新的 query.Run。
	Transcript []query.Message
	// CreatedAt 是 Agent 启动的时间戳。
	CreatedAt time.Time
}

// Registry 是线程安全的子 Agent 注册表，映射 ID → Record。
type Registry struct {
	mu      sync.RWMutex
	records map[ID]*Record
}

// NewRegistry 创建并返回一个空的 Registry。
func NewRegistry() *Registry {
	return &Registry{records: make(map[ID]*Record)}
}

// Register 向注册表中添加一条 Agent 记录；ID 重复时覆盖。
func (r *Registry) Register(rec *Record) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.records[rec.ID] = rec
}

// Get 按 ID 查找 Agent 记录；不存在时返回 nil。
func (r *Registry) Get(id ID) *Record {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.records[id]
}

// Update 通过回调函数修改已有 Agent 记录的字段；记录不存在时返回 false。
func (r *Registry) Update(id ID, fn func(*Record)) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	rec, ok := r.records[id]
	if !ok {
		return false
	}
	fn(rec)
	return true
}

// FormatResult 将子 Agent 的执行结果格式化为 <result> XML 字符串。
//
// 父 Agent 的 queryLoop 将此字符串作为工具调用的返回值，
// TAOR 框架会将其包装为 role=tool 的消息注入对话历史，
// 从而让父 Agent LLM 在下一轮 Think 阶段看到子 Agent 的输出。
func FormatResult(id ID, description string, result query.Result) string {
	status := string(result.StopReason)
	if result.Err != nil {
		status = fmt.Sprintf("failed: %v", result.Err)
	}
	summary := fmt.Sprintf(`Agent "%s" %s（共 %d 轮）`, description, status, result.TurnCount)

	return fmt.Sprintf(`<result>
  <agent_id>%s</agent_id>
  <status>%s</status>
  <status_summary>%s</status_summary>
  <result_text>%s</result_text>
</result>`, id, status, summary, result.Answer)
}

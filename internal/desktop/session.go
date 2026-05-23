package desktop

import (
	"context"
	"fmt"
	"matrix/internal/audit"
	"matrix/internal/logger"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"matrix/internal/agent"
	"matrix/internal/coordinator"
	"matrix/internal/matrixpaths"
	"matrix/internal/query"
	"matrix/internal/stream"
	"matrix/internal/tools"
)

const streamEventName = "agent:stream"

// sessionRunner 管理单次 Agent 会话的取消与流式推送。
type sessionRunner struct {
	mu        sync.Mutex
	runCancel context.CancelFunc
	sessionID string
	hub       *coordinator.StreamHub
	sidechain *agent.SidechainWriter
	audit     *audit.Writer
}

// persistChatFn 在 Agent 成功结束后持久化多轮 transcript。
type persistChatFn func(query.Result) error

// runAgentSession 执行单次 Agent 运行。chatSessionID 非空时同时用作流式事件与 audit 的 sessionID
// （与 ChatTranscriptStore 的 chatSessionID 键相同）；单轮任务传空字符串时自动生成独立 UUID。
func (b *Bridge) runAgentSession(initial []query.Message, chatSessionID string, persist persistChatFn) (*RunResult, error) {
	if b.sessions == nil {
		b.sessions = &sessionRunner{}
	}
	if b.client == nil {
		return nil, errNoAPIKey()
	}
	if len(initial) == 0 {
		return nil, fmt.Errorf("会话消息不能为空")
	}

	b.sessions.mu.Lock()
	if b.sessions.runCancel != nil {
		b.sessions.runCancel()
	}
	runCtx, cancel := context.WithCancel(b.ctx)
	b.sessions.runCancel = cancel
	sessionID := chatSessionID
	if sessionID == "" {
		sessionID = uuid.NewString()
	}
	b.sessions.sessionID = sessionID
	b.sessions.mu.Unlock()

	defer func() {
		b.sessions.mu.Lock()
		b.sessions.runCancel = nil
		b.sessions.mu.Unlock()
		cancel()
	}()

	base := wailsSink{emit: func(msg stream.Message) {
		if msg.SessionID == "" {
			msg.SessionID = sessionID
		}
		runtime.EventsEmit(b.ctx, streamEventName, msg)
	}}
	coalesced := newCoalesceSink(base, sessionID, 100*time.Millisecond)
	defer coalesced.close()

	ws := b.workspaceRoot()
	tools.SetWorkspaceRoot(ws)
	sidechain := agent.NewSidechainWriter(matrixpaths.SubagentsDir(ws))
	auditWriter := audit.NewWriter(ws, sessionID)
	taskPreview := audit.Preview(initial[len(initial)-1].Content, 500)
	auditWriter.UpdateMeta(audit.SessionMeta{
		Workspace:   ws,
		Model:       b.config.Model.Model,
		TaskPreview: taskPreview,
	})
	auditWriter.Emit("session.start", 0, "desktop", map[string]any{
		"task_preview":    taskPreview,
		"model":           b.config.Model.Model,
		"workspace":       ws,
		"chat_session_id": chatSessionID,
		"history_len":     len(initial),
	})

	hub := coordinator.NewStreamHub(
		sessionID,
		b.subAgentRegistry,
		sidechain,
		coalesced,
		nil,
		func(snap agent.Snapshot) {
			runtime.EventsEmit(b.ctx, subAgentUpdateEvent, toSubAgentDTO(snap))
		},
		func(snap agent.Snapshot) {
			runtime.EventsEmit(b.ctx, subAgentDoneEvent, toSubAgentDTO(snap))
		},
	)
	hub.Audit = auditWriter

	b.sessions.mu.Lock()
	b.sessions.hub = hub
	b.sessions.sidechain = sidechain
	b.sessions.audit = auditWriter
	b.sessions.mu.Unlock()
	defer func() {
		b.sessions.mu.Lock()
		b.sessions.hub = nil
		b.sessions.sidechain = nil
		b.sessions.audit = nil
		b.sessions.mu.Unlock()
	}()

	runCtx = logger.With(runCtx, logger.Fields{SessionID: sessionID, Component: "desktop"})
	logger.InfoCtx(runCtx, "SessionRunner: start",
		"session_id", sessionID,
		"chat_session_id", chatSessionID,
		"initial_messages", len(initial),
	)
	b.subAgentRegistry = coordinator.NewRegistry()
	if b.workerRun != nil {
		b.workerRun.SetParent(runCtx)
		defer b.workerRun.SetParent(context.Background())
	}
	cfg, err := b.buildQueryConfig(initial, sessionID, hub, auditWriter)
	if err != nil {
		return nil, err
	}
	start := time.Now()
	result := query.RunSession(runCtx, cfg, coalesced)
	dur := time.Since(start)
	errMsg := ""
	if result.Err != nil {
		errMsg = result.Err.Error()
	}
	auditWriter.Emit("session.end", 0, "desktop", map[string]any{
		"stop_reason": string(result.StopReason),
		"turns":       result.TurnCount,
		"duration_ms": dur.Milliseconds(),
		"error":       errMsg,
	})
	_ = auditWriter.Close(audit.SessionMeta{
		StopReason: string(result.StopReason),
		TurnCount:  result.TurnCount,
		DurationMs: dur.Milliseconds(),
		Error:      errMsg,
	})

	if persist != nil && result.Err == nil && result.StopReason != query.StopAborted && len(result.Messages) > 0 {
		if err := persist(result); err != nil {
			logger.Warnf("chat transcript persist failed: %v", err)
		}
	}

	return b.toRunResult(result)
}

// CancelAgentSession 取消当前正在运行的 Agent 会话。
func (b *Bridge) CancelAgentSession() error {
	b.sessions.mu.Lock()
	defer b.sessions.mu.Unlock()
	if b.sessions.runCancel == nil {
		return nil
	}
	b.sessions.runCancel()
	logger.Info("SessionRunner: cancelled")
	return nil
}

func errNoAPIKey() error {
	return fmt.Errorf("请先配置 API Key")
}

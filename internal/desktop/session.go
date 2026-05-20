package desktop

import (
	"context"
	"fmt"
	"matrix/internal/logger"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"matrix/internal/query"
	"matrix/internal/stream"
)

const streamEventName = "agent:stream"

// sessionRunner 管理单次 Agent 会话的取消与流式推送。
type sessionRunner struct {
	mu        sync.Mutex
	runCancel context.CancelFunc
	sessionID string
}

func (b *Bridge) runAgentSession(task string) (*RunResult, error) {
	if b.sessions == nil {
		b.sessions = &sessionRunner{}
	}
	if b.client == nil {
		return nil, errNoAPIKey()
	}

	b.sessions.mu.Lock()
	if b.sessions.runCancel != nil {
		b.sessions.runCancel()
	}
	runCtx, cancel := context.WithCancel(b.ctx)
	b.sessions.runCancel = cancel
	sessionID := uuid.NewString()
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

	logger.Infof("SessionRunner: start session=%s", sessionID)
	if b.workerRun != nil {
		b.workerRun.SetParent(runCtx)
		defer b.workerRun.SetParent(context.Background())
	}
	cfg, err := b.buildQueryConfig(b.formatUserMessage(task))
	if err != nil {
		return nil, err
	}
	cfg.SessionID = sessionID
	return b.toRunResult(query.RunSession(runCtx, cfg, coalesced))
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

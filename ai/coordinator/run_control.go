package coordinator

import (
	"context"
	"matrix/ai/agent"
	"sync"
)

// RunControl 跟踪运行中 Worker 的 context.CancelFunc，供 task_stop 与父会话取消使用。
//
// Worker 使用 Begin 得到的上下文，不直接使用父 TAOR 工具调用的 ctx，从而：
//   - task_stop 可单独终止某个 Worker；
//   - SetParent(sessionCtx) 在会话取消时终止该会话下所有 Worker。
type RunControl struct {
	mu      sync.Mutex
	parent  context.Context
	cancels map[agent.ID]context.CancelFunc
}

// NewRunControl 创建空的 RunControl；默认父上下文为 Background。
func NewRunControl() *RunControl {
	return &RunControl{
		parent:  context.Background(),
		cancels: make(map[agent.ID]context.CancelFunc),
	}
}

// SetParent 设置派生 Worker 上下文所依附的父 context（通常为会话 runCtx）。
func (rc *RunControl) SetParent(ctx context.Context) {
	if rc == nil {
		return
	}
	rc.mu.Lock()
	defer rc.mu.Unlock()
	if ctx == nil {
		rc.parent = context.Background()
		return
	}
	rc.parent = ctx
}

// Begin 为 id 注册可取消的 Worker 上下文。cleanup 必须在 query.Run 返回后调用（含 panic 路径）。
func (rc *RunControl) Begin(id agent.ID) (context.Context, func()) {
	if rc == nil {
		return context.Background(), func() {}
	}
	rc.mu.Lock()
	defer rc.mu.Unlock()
	parent := rc.parent
	if parent == nil {
		parent = context.Background()
	}
	workerCtx, cancel := context.WithCancel(parent)
	rc.cancels[id] = cancel
	return workerCtx, func() {
		rc.mu.Lock()
		defer rc.mu.Unlock()
		delete(rc.cancels, id)
	}
}

// Stop 取消正在运行的 Worker；若 id 未在运行中则返回 false。
func (rc *RunControl) Stop(id agent.ID) bool {
	if rc == nil {
		return false
	}
	rc.mu.Lock()
	cancel, ok := rc.cancels[id]
	rc.mu.Unlock()
	if !ok {
		return false
	}
	cancel()
	return true
}

// IsRunning 报告 id 是否仍有未结束的 Worker 执行（已注册 cancel 且尚未 cleanup）。
func (rc *RunControl) IsRunning(id agent.ID) bool {
	if rc == nil {
		return false
	}
	rc.mu.Lock()
	defer rc.mu.Unlock()
	_, ok := rc.cancels[id]
	return ok
}

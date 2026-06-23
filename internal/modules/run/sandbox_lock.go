package run

import (
	"sync"

	"github.com/google/uuid"
)

// sandboxLocks 按项目或「项目+仓库」串行化沙箱写入，不同仓库可并行。
type sandboxLocks struct {
	mu    sync.Mutex
	locks map[string]*sync.Mutex
}

// newSandboxLocks 创建沙箱互斥锁管理器。
func newSandboxLocks() *sandboxLocks {
	return &sandboxLocks{locks: make(map[string]*sync.Mutex)}
}

// sandboxLockKey 生成项目/仓库沙箱互斥锁键（项目编码 + 可选仓库 ID）。
func sandboxLockKey(projectCode string, repositoryID *uuid.UUID) string {
	if repositoryID == nil || *repositoryID == uuid.Nil {
		return projectCode
	}
	return projectCode + "/" + repositoryID.String()
}

// acquire 获取沙箱锁并返回解锁函数。
func (p *sandboxLocks) acquire(projectCode string, repositoryID *uuid.UUID) func() {
	key := sandboxLockKey(projectCode, repositoryID)
	p.mu.Lock()
	m, ok := p.locks[key]
	if !ok {
		m = &sync.Mutex{}
		p.locks[key] = m
	}
	p.mu.Unlock()
	m.Lock()
	return m.Unlock
}

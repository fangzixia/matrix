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

func newSandboxLocks() *sandboxLocks {
	return &sandboxLocks{locks: make(map[string]*sync.Mutex)}
}

func (p *sandboxLocks) acquire(projectID uuid.UUID, repositoryID *uuid.UUID) func() {
	key := sandboxLockKey(projectID, repositoryID)
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

// Package job 异步任务队列：Run 入队、重试与嵌入式 Worker 消费。
package job

import (
	"context"
	"errors"
	"matrix/internal/platform/db/models"
	"matrix/internal/platform/logging"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	jobStatusQueued  = "queued"
	jobStatusRunning = "running"
	jobStatusDone    = "done"
	jobStatusFailed  = "failed"

	defaultMaxAttempts  = 3
	defaultPollInterval = 2 * time.Second
	defaultConcurrency  = 1
)

// claimJobSQL 认领最早一条 queued 任务（SKIP LOCKED 避免 Worker 互斥等待）。
const claimJobSQL = `
	UPDATE run_jobs
	SET status = ?, locked_by = ?, locked_at = NOW(), attempts = attempts + 1, updated_at = NOW()
	WHERE id = (
		SELECT id FROM run_jobs
		WHERE status = ? AND attempts < ?
		ORDER BY created_at ASC
		LIMIT 1
		FOR UPDATE SKIP LOCKED
	)
	RETURNING id, run_id
`

// Executor 执行已认领的 Run 任务。
type Executor interface {
	ExecuteRun(ctx context.Context, runID uuid.UUID) error
}

// Service 管理异步任务队列：Run 入队、认领、完成与重试。
type Service struct {
	db          *gorm.DB
	maxAttempts int
}

// NewService 创建任务队列服务实例。
func NewService(db *gorm.DB, maxAttempts int) *Service {
	if maxAttempts <= 0 {
		maxAttempts = defaultMaxAttempts
	}
	return &Service{db: db, maxAttempts: maxAttempts}
}

// Enqueue 将 Run 任务入队。
func (s *Service) Enqueue(ctx context.Context, runID uuid.UUID) error {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return s.EnqueueTx(ctx, tx, runID)
	})
	if err == nil {
		logging.Info("job: 任务已入队", "run_id", runID)
	}
	return err
}

// EnqueueTx 在调用方事务内将 Run 任务入队。
func (s *Service) EnqueueTx(ctx context.Context, tx *gorm.DB, runID uuid.UUID) error {
	if err := tx.WithContext(ctx).Model(&models.Run{}).Where("id = ?", runID).Update("status", jobStatusQueued).Error; err != nil {
		return err
	}
	job := models.RunJob{RunID: runID, Status: jobStatusQueued}
	return tx.WithContext(ctx).Create(&job).Error
}

// ClaimedJob 是 Worker 认领到的待执行任务。
type ClaimedJob struct {
	JobID uuid.UUID
	RunID uuid.UUID
}

// Claim Worker 认领待执行任务。无可用任务时返回 (nil, nil)。
func (s *Service) Claim(ctx context.Context, workerID string) (*ClaimedJob, error) {
	var claimed *ClaimedJob
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var job models.RunJob
		res := tx.Raw(claimJobSQL, jobStatusRunning, workerID, jobStatusQueued, s.maxAttempts).Scan(&job)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 || job.ID == uuid.Nil {
			return gorm.ErrRecordNotFound
		}
		claimed = &ClaimedJob{JobID: job.ID, RunID: job.RunID}
		return nil
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return claimed, nil
}

// Complete 标记任务已完成。
func (s *Service) Complete(ctx context.Context, jobID uuid.UUID, success bool) error {
	status := jobStatusDone
	if !success {
		status = jobStatusFailed
	}
	return s.db.WithContext(ctx).Model(&models.RunJob{}).Where("id = ?", jobID).
		Updates(map[string]any{"status": status, "locked_by": "", "locked_at": nil}).Error
}

// Requeue 将失败任务重新入队。
func (s *Service) Requeue(ctx context.Context, jobID uuid.UUID) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var job models.RunJob
		if err := tx.First(&job, "id = ?", jobID).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.Run{}).Where("id = ?", job.RunID).Updates(map[string]any{
			"status":        jobStatusQueued,
			"error_message": "",
			"finished_at":   nil,
		}).Error; err != nil {
			return err
		}
		return tx.Model(&models.RunJob{}).Where("id = ?", jobID).
			Updates(map[string]any{"status": jobStatusQueued, "locked_by": "", "locked_at": nil}).Error
	})
}

// RunWorker 启动嵌入式任务 Worker 循环。
func (s *Service) RunWorker(ctx context.Context, workerID string, pollInterval time.Duration, concurrency int, exec Executor) {
	pollInterval, concurrency = normalizeWorkerOptions(pollInterval, concurrency)
	sem := make(chan struct{}, concurrency)

	for {
		if ctx.Err() != nil {
			logging.Info("job: Worker 已停止", "worker_id", workerID, "reason", "context_cancelled")
			return
		}

		claimed, err := s.Claim(ctx, workerID)
		if err != nil {
			logging.Warn("job: 认领失败", "worker_id", workerID, "error", err.Error())
			sleepOrDone(ctx, pollInterval)
			continue
		}
		if claimed == nil {
			sleepOrDone(ctx, pollInterval)
			continue
		}

		if !s.acquireSlot(ctx, sem, claimed) {
			return
		}
		go s.runClaimedJob(ctx, workerID, claimed, exec, sem)
	}
}

func normalizeWorkerOptions(pollInterval time.Duration, concurrency int) (time.Duration, int) {
	if pollInterval <= 0 {
		pollInterval = defaultPollInterval
	}
	if concurrency <= 0 {
		concurrency = defaultConcurrency
	}
	return pollInterval, concurrency
}

func sleepOrDone(ctx context.Context, d time.Duration) {
	select {
	case <-ctx.Done():
	case <-time.After(d):
	}
}

// acquireSlot 等待并发槽位。context 取消时将任务重新入队并返回 false。
func (s *Service) acquireSlot(ctx context.Context, sem chan struct{}, claimed *ClaimedJob) bool {
	logging.Info("job: 等待执行槽位",
		"run_id", claimed.RunID, "job_id", claimed.JobID,
		"concurrency", cap(sem),
	)
	select {
	case sem <- struct{}{}:
		return true
	case <-ctx.Done():
		logging.Warn("job: 槽位等待中断，任务将重新入队",
			"run_id", claimed.RunID, "job_id", claimed.JobID,
		)
		s.requeueOrWarn(ctx, claimed.JobID)
		return false
	}
}

func (s *Service) runClaimedJob(ctx context.Context, workerID string, c *ClaimedJob, exec Executor, sem chan struct{}) {
	defer func() { <-sem }()

	logging.Info("job: 已认领任务", "run_id", c.RunID, "job_id", c.JobID, "worker_id", workerID)
	if ctx.Err() != nil {
		logging.Info("job: 执行前 context 已取消", "run_id", c.RunID, "job_id", c.JobID)
		s.requeueOrWarn(ctx, c.JobID)
		return
	}

	start := time.Now()
	runErr := exec.ExecuteRun(ctx, c.RunID)
	logging.Info("job: 执行完成",
		"run_id", c.RunID, "job_id", c.JobID,
		"duration_ms", time.Since(start).Milliseconds(),
		"success", runErr == nil,
	)
	s.finishJob(ctx, c.JobID, runErr)
}

// finishJob 更新任务终态；失败且未达最大重试次数时重新入队。
func (s *Service) finishJob(ctx context.Context, jobID uuid.UUID, runErr error) {
	success := runErr == nil
	if err := s.Complete(ctx, jobID, success); err != nil {
		logging.Warn("job: 更新任务状态失败", "job_id", jobID, "error", err.Error())
		return
	}
	if success {
		return
	}
	var job models.RunJob
	if err := s.db.WithContext(ctx).First(&job, "id = ?", jobID).Error; err != nil {
		logging.Warn("job: 读取任务重试次数失败", "job_id", jobID, "error", err.Error())
		return
	}
	if job.Attempts >= s.maxAttempts {
		return
	}
	s.requeueOrWarn(ctx, jobID)
}

func (s *Service) requeueOrWarn(ctx context.Context, jobID uuid.UUID) {
	if err := s.Requeue(ctx, jobID); err != nil {
		logging.Warn("job: 重新入队失败", "job_id", jobID, "error", err.Error())
	}
}

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
		maxAttempts = 3
	}
	return &Service{db: db, maxAttempts: maxAttempts}
}

// Enqueue 将 Run 任务入队。
func (s *Service) Enqueue(ctx context.Context, runID uuid.UUID) error {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.Run{}).Where("id = ?", runID).Update("status", "queued").Error; err != nil {
			return err
		}
		job := models.RunJob{RunID: runID, Status: "queued"}
		return tx.Create(&job).Error
	})
	if err == nil {
		logging.Info("job: 任务已入队", "run_id", runID)
	}
	return err
}

// ClaimedJob 是 Worker 认领到的待执行任务。
type ClaimedJob struct {
	JobID uuid.UUID
	RunID uuid.UUID
}

// Claim Worker 认领待执行任务。
func (s *Service) Claim(ctx context.Context, workerID string) (*ClaimedJob, error) {
	var claimed *ClaimedJob
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var job models.RunJob
		res := tx.Raw(`
			UPDATE run_jobs
			SET status = 'running', locked_by = ?, locked_at = NOW(), attempts = attempts + 1, updated_at = NOW()
			WHERE id = (
				SELECT id FROM run_jobs
				WHERE status = 'queued' AND attempts < ?
				ORDER BY created_at ASC
				LIMIT 1
				FOR UPDATE SKIP LOCKED
			)
			RETURNING id, run_id
		`, workerID, s.maxAttempts).Scan(&job)
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
	status := "done"
	if !success {
		status = "failed"
	}
	return s.db.WithContext(ctx).Model(&models.RunJob{}).Where("id = ?", jobID).
		Updates(map[string]any{"status": status, "locked_by": "", "locked_at": nil}).Error
}

// Requeue 将失败任务重新入队。
func (s *Service) Requeue(ctx context.Context, jobID uuid.UUID) error {
	return s.db.WithContext(ctx).Model(&models.RunJob{}).Where("id = ?", jobID).
		Updates(map[string]any{"status": "queued", "locked_by": "", "locked_at": nil}).Error
}

// RunWorker 启动嵌入式任务 Worker 循环。
func (s *Service) RunWorker(ctx context.Context, workerID string, pollInterval time.Duration, concurrency int, exec Executor) {
	if pollInterval <= 0 {
		pollInterval = 2 * time.Second
	}
	if concurrency <= 0 {
		concurrency = 1
	}
	sem := make(chan struct{}, concurrency)
	for {
		select {
		case <-ctx.Done():
			logging.Info("job: Worker 已停止", "worker_id", workerID, "reason", "context_cancelled")
			return
		default:
		}
		claimed, err := s.Claim(ctx, workerID)
		if err != nil {
			logging.Warn("job: 认领失败", "worker_id", workerID, "error", err.Error())
			time.Sleep(pollInterval)
			continue
		}
		if claimed == nil {
			time.Sleep(pollInterval)
			continue
		}
		logging.Info("job: 等待执行槽位",
			"run_id", claimed.RunID, "job_id", claimed.JobID,
			"worker_id", workerID, "concurrency", concurrency,
		)
		select {
		case sem <- struct{}{}:
		case <-ctx.Done():
			logging.Warn("job: 槽位等待中断，任务将重新入队",
				"run_id", claimed.RunID, "job_id", claimed.JobID,
			)
			_ = s.Requeue(ctx, claimed.JobID)
			return
		}
		go func(c *ClaimedJob) {
			defer func() { <-sem }()
			logging.Info("job: 已认领任务", "run_id", c.RunID, "job_id", c.JobID, "worker_id", workerID)
			if err := ctx.Err(); err != nil {
				logging.Warn("job: 执行前 context 已取消",
					"run_id", c.RunID, "job_id", c.JobID, "error", err.Error(),
				)
			}
			start := time.Now()
			runErr := exec.ExecuteRun(ctx, c.RunID)
			success := runErr == nil
			if success {
				logging.Info("job: 执行完成",
					"run_id", c.RunID, "job_id", c.JobID,
					"duration_ms", time.Since(start).Milliseconds(),
				)
			} else {
				logging.Warn("job: 执行失败",
					"run_id", c.RunID, "job_id", c.JobID,
					"duration_ms", time.Since(start).Milliseconds(),
					"error", runErr.Error(),
				)
			}
			_ = s.Complete(ctx, c.JobID, success)
			if !success && runErr != nil {
				var job models.RunJob
				if s.db.WithContext(ctx).First(&job, "id = ?", c.JobID).Error == nil && job.Attempts < s.maxAttempts {
					_ = s.Requeue(ctx, c.JobID)
				}
			}
		}(claimed)
	}
}

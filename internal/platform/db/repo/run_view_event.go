package repo

import (
	"context"

	"matrix/internal/platform/db/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// RunViewEventRepo 封装 run_view_events 表持久化。
type RunViewEventRepo struct {
	db *gorm.DB
}

// NewRunViewEventRepo 创建 RunViewEventRepo。
func NewRunViewEventRepo(db *gorm.DB) *RunViewEventRepo {
	return &RunViewEventRepo{db: db}
}

// Append 追加一条事件日志（seq 由调用方分配）。
func (r *RunViewEventRepo) Append(ctx context.Context, row *models.RunViewEvent) error {
	return r.db.WithContext(ctx).Create(row).Error
}

// ListAfterSeq 返回 job_id 下 seq > afterSeq 的事件，按 seq 升序。
func (r *RunViewEventRepo) ListAfterSeq(ctx context.Context, jobID uuid.UUID, afterSeq int64) ([]models.RunViewEvent, error) {
	var rows []models.RunViewEvent
	err := r.db.WithContext(ctx).
		Where("job_id = ? AND seq > ?", jobID, afterSeq).
		Order("seq ASC").
		Find(&rows).Error
	return rows, err
}

// MaxSeq 返回 job_id 下最大 seq；无记录时返回 0。
func (r *RunViewEventRepo) MaxSeq(ctx context.Context, jobID uuid.UUID) (int64, error) {
	var max *int64
	err := r.db.WithContext(ctx).
		Model(&models.RunViewEvent{}).
		Where("job_id = ?", jobID).
		Select("MAX(seq)").
		Scan(&max).Error
	if err != nil || max == nil {
		return 0, err
	}
	return *max, nil
}

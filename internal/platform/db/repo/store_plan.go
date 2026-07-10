package repo

import (
	"context"
	"time"

	"matrix/internal/platform/db/models"

	"github.com/google/uuid"
)

// PlanStore 封装计划文档索引持久化。
type PlanStore struct {
	c *catalog
}

func newPlanStore(c *catalog) *PlanStore { return &PlanStore{c: c} }

const (
	PlanStatusDraft    = "draft"
	PlanStatusApproved = "approved"
)

// IsApproved 检查计划是否已批准。
func (s *PlanStore) IsApproved(ctx context.Context, projectID uuid.UUID, path string) (bool, error) {
	row, err := s.c.plan.GetByProjectAndPath(ctx, projectID, path)
	if IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return row.Status == PlanStatusApproved, nil
}

// PlanStatus 返回计划批准状态。
func (s *PlanStore) PlanStatus(ctx context.Context, projectID uuid.UUID, path string) (string, error) {
	row, err := s.c.plan.GetByProjectAndPath(ctx, projectID, path)
	if IsNotFound(err) {
		return PlanStatusDraft, nil
	}
	if err != nil {
		return "", err
	}
	if row.Status == "" {
		return PlanStatusDraft, nil
	}
	return row.Status, nil
}

// ApproveParams 批准计划参数。
type ApproveParams struct {
	ProjectID   uuid.UUID
	Path        string
	Title       string
	Resolutions map[string]string
}

// Approve 原子批准计划并写入确认项。
func (s *PlanStore) Approve(ctx context.Context, p ApproveParams) error {
	now := time.Now()
	row, err := s.c.plan.GetByProjectAndPath(ctx, p.ProjectID, p.Path)
	createNew := IsNotFound(err)
	if createNew {
		row = &models.Plan{
			ID: uuid.New(), ProjectID: p.ProjectID, Path: p.Path,
			Title: p.Title, Status: PlanStatusApproved,
			CreatedAt: now, UpdatedAt: now,
		}
	} else if err != nil {
		return err
	} else {
		row.Status = PlanStatusApproved
		row.UpdatedAt = now
		if row.Title == "" {
			row.Title = p.Title
		}
	}
	return s.c.plan.ApproveWithResolutions(ctx, row, createNew, p.Resolutions)
}

// IndexAfterRunParams Run 成功后索引计划参数。
type IndexAfterRunParams struct {
	ProjectID    uuid.UUID
	RepositoryID *uuid.UUID
	RunID        uuid.UUID
	Path         string
	Title        string
}

// IndexAfterRun 在 plan 阶段成功后写入/更新计划索引。
func (s *PlanStore) IndexAfterRun(ctx context.Context, p IndexAfterRunParams) error {
	now := time.Now()
	row := models.Plan{
		ProjectID: p.ProjectID, RepositoryID: p.RepositoryID, RunID: &p.RunID,
		Path: p.Path, Title: p.Title, Status: PlanStatusDraft,
		UpdatedAt: now, CreatedAt: now,
	}
	return s.c.plan.UpsertAfterRun(ctx, &row)
}

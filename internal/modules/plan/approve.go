package plan

import (
	"context"
	"errors"
	"fmt"
	"matrix/internal/modules/docmeta"
	"matrix/internal/modules/workspace"
	"matrix/internal/platform/db/models"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	StatusDraft    = "draft"
	StatusApproved = "approved"
)

// ConfirmInput 是用户确认计划项的请求。
type ConfirmInput struct {
	Path        string            `json:"path"`
	Resolutions map[string]string `json:"resolutions"`
}

// IsApproved 检查计划是否已批准。
func (s *Service) IsApproved(ctx context.Context, projectID uuid.UUID, logicalPath string) (bool, error) {
	path, err := workspace.SanitizeDocLogicalPath(strings.TrimSpace(logicalPath))
	if err != nil || path == "" {
		return false, fmt.Errorf("无效的计划路径")
	}
	var row models.Plan
	err = s.db.WithContext(ctx).Where("project_id = ? AND path = ?", projectID, path).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return row.Status == StatusApproved, nil
}

func (s *Service) savePlanResolutions(ctx context.Context, planID uuid.UUID, resolutions map[string]string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("plan_id = ?", planID).Delete(&models.PlanResolution{}).Error; err != nil {
			return err
		}
		for key, val := range resolutions {
			key = strings.TrimSpace(key)
			if key == "" {
				continue
			}
			row := models.PlanResolution{
				PlanID: planID, ItemKey: key, Resolution: strings.TrimSpace(val),
			}
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// Approve 保存用户确认并批准计划。
func (s *Service) Approve(ctx context.Context, projectID uuid.UUID, in ConfirmInput) error {
	path, err := workspace.SanitizeDocLogicalPath(strings.TrimSpace(in.Path))
	if err != nil || path == "" {
		return fmt.Errorf("无效的计划路径")
	}
	full, err := s.ws.ResolveDocPath(projectID, path)
	if err != nil {
		return err
	}
	content, err := os.ReadFile(full)
	if err != nil {
		return err
	}
	now := time.Now()
	var row models.Plan
	err = s.db.WithContext(ctx).Where("project_id = ? AND path = ?", projectID, path).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		row = models.Plan{
			ID: uuid.New(), ProjectID: projectID, Path: path,
			Title:     docmeta.TitleOrFallback(path, string(content)),
			Status:    StatusApproved,
			CreatedAt: now, UpdatedAt: now,
		}
		if err := s.db.WithContext(ctx).Create(&row).Error; err != nil {
			return err
		}
		return s.savePlanResolutions(ctx, row.ID, in.Resolutions)
	}
	if err != nil {
		return err
	}
	row.Status = StatusApproved
	row.UpdatedAt = now
	if row.Title == "" {
		row.Title = docmeta.TitleOrFallback(path, string(content))
	}
	if err := s.db.WithContext(ctx).Save(&row).Error; err != nil {
		return err
	}
	return s.savePlanResolutions(ctx, row.ID, in.Resolutions)
}

// PlanStatus 返回计划批准状态。
func (s *Service) PlanStatus(ctx context.Context, projectID uuid.UUID, logicalPath string) (string, error) {
	path, err := workspace.SanitizeDocLogicalPath(strings.TrimSpace(logicalPath))
	if err != nil || path == "" {
		return StatusDraft, nil
	}
	var row models.Plan
	err = s.db.WithContext(ctx).Where("project_id = ? AND path = ?", projectID, path).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return StatusDraft, nil
	}
	if err != nil {
		return "", err
	}
	if row.Status == "" {
		return StatusDraft, nil
	}
	return row.Status, nil
}

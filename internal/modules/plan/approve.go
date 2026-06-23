package plan

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	items := ParseSectionItems(string(content))
	unresolved := UnresolvedKeys(items, in.Resolutions)
	if len(unresolved) > 0 {
		return fmt.Errorf("仍有未确认项: %s", strings.Join(unresolved, "; "))
	}
	resJSON, _ := json.Marshal(in.Resolutions)
	now := time.Now()
	var row models.Plan
	err = s.db.WithContext(ctx).Where("project_id = ? AND path = ?", projectID, path).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		row = models.Plan{
			ProjectID: projectID, Path: path,
			Title:  titleOrFromContent(path, string(content)),
			Status: StatusApproved, Resolutions: string(resJSON),
			CreatedAt: now, UpdatedAt: now,
		}
		return s.db.WithContext(ctx).Create(&row).Error
	}
	if err != nil {
		return err
	}
	row.Status = StatusApproved
	row.Resolutions = string(resJSON)
	row.UpdatedAt = now
	if row.Title == "" {
		row.Title = titleOrFromContent(path, string(content))
	}
	return s.db.WithContext(ctx).Save(&row).Error
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

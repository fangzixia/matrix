package plan

import (
	"context"
	"fmt"
	"matrix/internal/modules/docmeta"
	"matrix/internal/modules/workspace"
	"matrix/internal/platform/db/repo"
	"os"
	"strings"

	"github.com/google/uuid"
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
	return s.stores.Plan.IsApproved(ctx, projectID, path)
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
	title := docmeta.TitleOrFallback(path, string(content))
	return s.stores.Plan.Approve(ctx, repo.ApproveParams{
		ProjectID: projectID, Path: path, Title: title, Resolutions: in.Resolutions,
	})
}

// PlanStatus 返回计划批准状态。
func (s *Service) PlanStatus(ctx context.Context, projectID uuid.UUID, logicalPath string) (string, error) {
	path, err := workspace.SanitizeDocLogicalPath(strings.TrimSpace(logicalPath))
	if err != nil || path == "" {
		return StatusDraft, nil
	}
	st, err := s.stores.Plan.PlanStatus(ctx, projectID, path)
	if err != nil {
		return "", err
	}
	if st == repo.PlanStatusDraft {
		return StatusDraft, nil
	}
	if st == repo.PlanStatusApproved {
		return StatusApproved, nil
	}
	return st, nil
}

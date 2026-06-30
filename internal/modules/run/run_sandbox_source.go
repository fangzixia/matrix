package run

import (
	"context"
	"errors"
	"fmt"
	"matrix/internal/platform/db/models"
	"matrix/internal/platform/logging"
	"os"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var codeSandboxKinds = []string{"implement", "pipeline", "build"}

// latestCodeSandboxForPlan 返回同一计划下最近一次成功产出代码的 Run 沙箱。
func (s *Service) latestCodeSandboxForPlan(
	ctx context.Context,
	projectID uuid.UUID,
	repositoryID *uuid.UUID,
	planPath string,
	excludeRunID uuid.UUID,
) (sandboxPath string, sourceRunID uuid.UUID, err error) {
	planPath = strings.TrimSpace(planPath)
	if planPath == "" {
		return "", uuid.Nil, errors.New("计划路径为空")
	}
	q := s.db.WithContext(ctx).Model(&models.Run{}).
		Where("project_id = ?", projectID).
		Where("file_path = ?", planPath).
		Where("kind IN ?", codeSandboxKinds).
		Where("status = ?", "succeeded").
		Where("sandbox_path <> ''").
		Where("id <> ?", excludeRunID)
	if repositoryID != nil {
		q = q.Where("repository_id = ?", *repositoryID)
	}
	var row models.Run
	if err := q.Order("finished_at DESC NULLS LAST, created_at DESC").First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", uuid.Nil, fmt.Errorf("未找到计划 %s 的实现 Run，请先完成 implement、pipeline 或 build", planPath)
		}
		return "", uuid.Nil, err
	}
	if !dirExists(row.SandboxPath) {
		return "", uuid.Nil, fmt.Errorf("计划 %s 的实现 Run %s 沙箱已不存在，请重新执行 implement", planPath, row.ID)
	}
	return row.SandboxPath, row.ID, nil
}

func (s *Service) resolveVerifySandbox(ctx context.Context, m *models.Run) (string, uuid.UUID, error) {
	sourcePath, sourceRunID, err := s.latestCodeSandboxForPlan(ctx, m.ProjectID, m.RepositoryID, m.FilePath, m.ID)
	if err != nil {
		return "", uuid.Nil, err
	}
	sandboxPath, err := s.workspace.CopyRunRepoFrom(ctx, m.ProjectID, sourcePath, m.ID)
	if err != nil {
		return "", uuid.Nil, err
	}
	logging.Agent("run: verify 从实现 Run 复制沙箱",
		"run_id", m.ID, "source_run_id", sourceRunID, "source_sandbox_path", sourcePath,
		"sandbox_path", sandboxPath, "file_path", m.FilePath,
	)
	return sandboxPath, sourceRunID, nil
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

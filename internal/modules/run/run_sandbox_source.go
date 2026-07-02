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

// 可产出代码的沙箱来源 Run 类型（implement 多次执行时复用）。
var codeSandboxKinds = []string{"implement", "pipeline", "build"}

// verify 评测时仅复用最近一次成功的 implement Run。
var verifySourceKinds = []string{"implement"}

type copyRepoOpts struct {
	stage         string
	sourceKinds   []string
	requireSource bool
	notFoundHint  string
}

// copyRepo 从同 plan 指定来源 Run 复制 repo 到当前 Run，避免重新 git clone。
// requireSource 为 false 时（implement）找不到来源不报错，由调用方回退 clone。
func (s *Service) copyRepo(
	ctx context.Context,
	m *models.Run,
	opts copyRepoOpts,
) (sandboxPath string, sourceRunID uuid.UUID, copied bool, err error) {
	sourcePath, sourceRunID, err := s.latestSandboxForPlan(
		ctx, m.ProjectID, m.RepositoryID, m.FilePath, m.ID, opts.sourceKinds, opts.notFoundHint,
	)
	if err != nil {
		if !opts.requireSource && isCodeSandboxNotFound(err) {
			return "", uuid.Nil, false, nil
		}
		return "", uuid.Nil, false, err
	}
	sandboxPath, err = s.workspace.CopyRepo(ctx, m.ProjectID, sourcePath, m.ID)
	if err != nil {
		return "", uuid.Nil, false, err
	}
	logging.Agent("run: "+opts.stage+" 从来源 Run 复制 repo",
		"run_id", m.ID, "source_run_id", sourceRunID, "source_sandbox_path", sourcePath,
		"sandbox_path", sandboxPath, "file_path", m.FilePath, "source_kinds", opts.sourceKinds,
	)
	return sandboxPath, sourceRunID, true, nil
}

// latestSandboxForPlan 返回同一计划下符合 kinds 的最近一次成功 Run 沙箱路径。
func (s *Service) latestSandboxForPlan(
	ctx context.Context,
	projectID uuid.UUID,
	repositoryID *uuid.UUID,
	planPath string,
	excludeRunID uuid.UUID,
	kinds []string,
	notFoundHint string,
) (sandboxPath string, sourceRunID uuid.UUID, err error) {
	planPath = strings.TrimSpace(planPath)
	if planPath == "" {
		return "", uuid.Nil, errors.New("计划路径为空")
	}
	if len(kinds) == 0 {
		return "", uuid.Nil, errors.New("来源 Run 类型为空")
	}
	if notFoundHint == "" {
		notFoundHint = "实现"
	}
	q := s.db.WithContext(ctx).Model(&models.Run{}).
		Where("project_id = ?", projectID).
		Where("file_path = ?", planPath).
		Where("kind IN ?", kinds).
		Where("status = ?", "succeeded").
		Where("sandbox_path <> ''").
		Where("id <> ?", excludeRunID)
	if repositoryID != nil {
		q = q.Where("repository_id = ?", *repositoryID)
	}
	var row models.Run
	if err := q.Order("finished_at DESC NULLS LAST, created_at DESC").First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", uuid.Nil, fmt.Errorf("未找到计划 %s 的 %s Run，请先完成 %s", planPath, notFoundHint, notFoundHint)
		}
		return "", uuid.Nil, err
	}
	if !dirExists(row.SandboxPath) {
		return "", uuid.Nil, fmt.Errorf("计划 %s 的 %s Run %s 沙箱已不存在，请重新执行 implement", planPath, notFoundHint, row.ID)
	}
	return row.SandboxPath, row.ID, nil
}

func isCodeSandboxNotFound(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, gorm.ErrRecordNotFound) || strings.Contains(err.Error(), "未找到计划")
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

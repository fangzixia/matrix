package workspace

import (
	"context"

	"github.com/google/uuid"
)

// CreateRunWorktreeFor 解析仓库名并创建 Run worktree。
func (s *Service) CreateRunWorktreeFor(ctx context.Context, projectID uuid.UUID, repositoryID *uuid.UUID, runID uuid.UUID) (sandboxPath, branch, repoName string, err error) {
	repoName, err = s.repoNameFor(ctx, projectID, repositoryID)
	if err != nil {
		return "", "", "", err
	}
	sandboxPath, branch, err = s.CreateRunWorktree(ctx, projectID, repoName, runID)
	return sandboxPath, branch, repoName, err
}

// RemoveRunWorktreeFor 删除指定 Run 的 worktree。
func (s *Service) RemoveRunWorktreeFor(ctx context.Context, projectID uuid.UUID, repositoryID *uuid.UUID, runID uuid.UUID, branch, sandboxPath string) error {
	repoName, err := s.repoNameFor(ctx, projectID, repositoryID)
	if err != nil {
		return err
	}
	return s.RemoveRunWorktree(ctx, projectID, repoName, runID, branch, sandboxPath)
}

// MergeRunWorktreeFor 将 Run worktree 合并回主仓库。
func (s *Service) MergeRunWorktreeFor(ctx context.Context, projectID uuid.UUID, repositoryID *uuid.UUID, runID uuid.UUID, branch, sandboxPath string) (*MergeResult, error) {
	repoName, err := s.repoNameFor(ctx, projectID, repositoryID)
	if err != nil {
		return nil, err
	}
	return s.MergeRunWorktree(ctx, projectID, repoName, runID, branch, sandboxPath)
}

func (s *Service) repoNameFor(ctx context.Context, projectID uuid.UUID, repositoryID *uuid.UUID) (string, error) {
	if repositoryID != nil && s.repos != nil {
		r, err := s.repos.Get(ctx, *repositoryID)
		if err != nil {
			return "", err
		}
		return r.Name, nil
	}
	if s.repos != nil {
		if r, err := s.repos.GetDefault(ctx, projectID); err == nil {
			return r.Name, nil
		}
	}
	return "default", nil
}

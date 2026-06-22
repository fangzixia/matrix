package workspace

import (
	"context"

	"github.com/google/uuid"

	"matrix/internal/modules/project"
	"matrix/internal/modules/repository"
)

// ProjectRepoResolver 根据项目与仓库 ID 解析沙箱根目录并确保仓库已克隆。
type ProjectRepoResolver struct {
	Projects *project.Service
	Repos    *repository.Service
	WS       *Service
}

// RepoRoot 返回项目默认仓库根目录。
func (r *ProjectRepoResolver) RepoRoot(ctx context.Context, projectID uuid.UUID) (string, error) {
	return r.RepoRootFor(ctx, projectID, nil)
}

// RepoRootFor 返回指定仓库的根目录。
func (r *ProjectRepoResolver) RepoRootFor(ctx context.Context, projectID uuid.UUID, repoID *uuid.UUID) (string, error) {
	var name, gitURL, branch string
	if repoID != nil && r.Repos != nil {
		repo, err := r.Repos.Get(ctx, *repoID)
		if err != nil {
			return "", err
		}
		name = repo.Name
		gitURL = repo.GitURL
		branch = repo.GitBranch
	} else if r.Repos != nil {
		repo, err := r.Repos.GetDefault(ctx, projectID)
		if err == nil {
			name = repo.Name
			gitURL = repo.GitURL
			branch = repo.GitBranch
		}
	}
	if name == "" {
		p, err := r.Projects.Get(ctx, projectID)
		if err != nil {
			return "", err
		}
		name = "default"
		gitURL = p.GitURL
		branch = p.GitBranch
	}
	if err := r.WS.EnsureRepo(ctx, projectID, name, gitURL, branch); err != nil {
		return "", err
	}
	return r.WS.NamedRepoRoot(projectID, name), nil
}

// CreateRunWorktree 创建 Run 专用 worktree 沙箱。
func (r *ProjectRepoResolver) CreateRunWorktree(ctx context.Context, projectID uuid.UUID, repositoryID *uuid.UUID, runID uuid.UUID) (sandboxPath, branch string, err error) {
	if _, err := r.RepoRootFor(ctx, projectID, repositoryID); err != nil {
		return "", "", err
	}
	sandboxPath, branch, _, err = r.WS.CreateRunWorktreeFor(ctx, projectID, repositoryID, runID)
	return sandboxPath, branch, err
}

// RemoveRunWorktree 删除 Run worktree。
func (r *ProjectRepoResolver) RemoveRunWorktree(ctx context.Context, projectID uuid.UUID, repositoryID *uuid.UUID, runID uuid.UUID, branch, sandboxPath string) error {
	return r.WS.RemoveRunWorktreeFor(ctx, projectID, repositoryID, runID, branch, sandboxPath)
}

// MergeRunWorktree 合并 Run worktree 到主仓库。
func (r *ProjectRepoResolver) MergeRunWorktree(ctx context.Context, projectID uuid.UUID, repositoryID *uuid.UUID, runID uuid.UUID, branch, sandboxPath string) ([]string, error) {
	res, err := r.WS.MergeRunWorktreeFor(ctx, projectID, repositoryID, runID, branch, sandboxPath)
	if err != nil && res != nil {
		return res.Conflicts, err
	}
	if err != nil {
		return nil, err
	}
	return nil, nil
}

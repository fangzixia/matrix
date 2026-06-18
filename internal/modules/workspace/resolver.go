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

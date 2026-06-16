package workspace

import (
	"context"

	"github.com/google/uuid"

	"matrix/internal/modules/project"
	"matrix/internal/modules/repository"
)

type ProjectRepoResolver struct {
	Projects *project.Service
	Repos    *repository.Service
	WS       *Service
}

func (r *ProjectRepoResolver) RepoRoot(ctx context.Context, projectID uuid.UUID) (string, error) {
	return r.RepoRootFor(ctx, projectID, nil)
}

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

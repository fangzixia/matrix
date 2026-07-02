package workspace

import (
	"context"
	"matrix/internal/modules/project"
	"matrix/internal/modules/repository"

	"github.com/google/uuid"
)

// ProjectRepoResolver 根据项目与仓库 ID 解析 Run 沙箱与文档目录。
type ProjectRepoResolver struct {
	Projects *project.Service
	Repos    *repository.Service
	WS       *Service
}

// ProjectWorkspaceKey 返回项目工作区目录键（项目编码）。
func (r *ProjectRepoResolver) ProjectWorkspaceKey(ctx context.Context, projectID uuid.UUID) (string, error) {
	return r.WS.ProjectWorkspaceKey(ctx, projectID)
}

// CreateRunRepo 为 Run 独立克隆 Git 仓库。
func (r *ProjectRepoResolver) CreateRunRepo(ctx context.Context, projectID uuid.UUID, repositoryID *uuid.UUID, runID uuid.UUID) (string, error) {
	gitURL, branch, err := r.gitConfigFor(ctx, projectID, repositoryID)
	if err != nil {
		return "", err
	}
	return r.WS.CreateRunRepo(ctx, projectID, runID, gitURL, branch)
}

// CopyRepo 将来源 repo 复制到目标 Run 沙箱。
func (r *ProjectRepoResolver) CopyRepo(ctx context.Context, projectID uuid.UUID, sourceRepoDir string, runID uuid.UUID) (string, error) {
	return r.WS.CopyRepo(ctx, projectID, sourceRepoDir, runID)
}

// RemoveRunRepo 删除 Run 沙箱目录。
func (r *ProjectRepoResolver) RemoveRunRepo(ctx context.Context, projectID uuid.UUID, runID uuid.UUID) error {
	return r.WS.RemoveRunRepo(ctx, projectID, runID)
}

func (r *ProjectRepoResolver) gitConfigFor(ctx context.Context, projectID uuid.UUID, repoID *uuid.UUID) (gitURL, branch string, err error) {
	if repoID != nil && r.Repos != nil {
		repo, err := r.Repos.GetForProject(ctx, projectID, *repoID)
		if err != nil {
			return "", "", err
		}
		return repo.GitURL, repo.GitBranch, nil
	}
	if r.Repos != nil {
		repo, err := r.Repos.GetDefault(ctx, projectID)
		if err == nil {
			return repo.GitURL, repo.GitBranch, nil
		}
	}
	p, err := r.Projects.Get(ctx, projectID)
	if err != nil {
		return "", "", err
	}
	return p.GitURL, p.GitBranch, nil
}

// DocsRoot 返回项目文档根目录并确保目录存在。
func (r *ProjectRepoResolver) DocsRoot(ctx context.Context, projectID uuid.UUID) (string, error) {
	if err := r.WS.EnsureDocsLayout(projectID); err != nil {
		return "", err
	}
	return r.WS.DocsRoot(projectID)
}

// ResolveDocPath 解析文档逻辑路径为绝对路径。
func (r *ProjectRepoResolver) ResolveDocPath(projectID uuid.UUID, logicalPath string) (string, error) {
	return r.WS.ResolveDocPath(projectID, logicalPath)
}

// SanitizeDocLogicalPath 校验文档逻辑路径。
func (r *ProjectRepoResolver) SanitizeDocLogicalPath(logicalPath string) (string, error) {
	return SanitizeDocLogicalPath(logicalPath)
}

// DocSandboxDir 返回 AI 可访问的文档沙箱根目录。
func (r *ProjectRepoResolver) DocSandboxDir(ctx context.Context, projectID uuid.UUID) (string, error) {
	_ = ctx
	return r.WS.DocSandboxDir(projectID)
}

// MatrixDir 返回单次 Run 的 Agent 运行时数据目录。
func (r *ProjectRepoResolver) MatrixDir(ctx context.Context, projectID, runID uuid.UUID) (string, error) {
	return r.WS.MatrixDir(ctx, projectID, runID)
}

// ListRunFiles 列出 Run 仓库目录树。
func (r *ProjectRepoResolver) ListRunFiles(ctx context.Context, projectID, runID uuid.UUID, rel string) ([]FileEntry, error) {
	return r.WS.ListRunFiles(ctx, projectID, runID, rel)
}

// ReadRunFile 读取 Run 仓库内文件。
func (r *ProjectRepoResolver) ReadRunFile(ctx context.Context, projectID, runID uuid.UUID, rel string) (string, error) {
	return r.WS.ReadRunFile(ctx, projectID, runID, rel)
}

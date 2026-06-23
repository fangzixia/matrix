// Package workspace Git 工作区：克隆、拉取、推送、文件树与多仓库解析。
package workspace

import (
	"context"
	"fmt"
	"matrix/internal/modules/repository"
	"matrix/internal/platform/config"
	"matrix/internal/platform/gitutil"
	"matrix/internal/platform/storage"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Service 管理 Git 工作区：克隆、拉取、推送、文件树与多仓库解析。
type Service struct {
	paths storage.Paths
	git   config.GitConfig
	repos *repository.Service
	keys  ProjectKeyResolver
}

// NewService 创建工作区服务实例。
func NewService(paths storage.Paths, git config.GitConfig, repos *repository.Service) *Service {
	return &Service{paths: paths, git: git, repos: repos}
}

// UpdateGit 更新工作区 Git 远程配置。
func (s *Service) UpdateGit(git config.GitConfig) {
	s.git = git
}

// SetProjectKeyResolver 注入项目工作区目录键解析器。
func (s *Service) SetProjectKeyResolver(r ProjectKeyResolver) {
	s.keys = r
}

// ProjectWorkspaceKey 返回项目工作区目录键（项目编码）。
func (s *Service) ProjectWorkspaceKey(ctx context.Context, projectID uuid.UUID) (string, error) {
	if s.keys == nil {
		return "", fmt.Errorf("workspace: project key resolver not configured")
	}
	return s.keys.ProjectWorkspaceKey(ctx, projectID)
}

// RepoRoot 返回项目默认仓库根目录。
func (s *Service) RepoRoot(projectID uuid.UUID) (string, error) {
	return s.namedRepoRoot(context.Background(), projectID, "")
}

// NamedRepoRoot 按仓库名返回根目录。
func (s *Service) NamedRepoRoot(projectID uuid.UUID, repoName string) (string, error) {
	return s.namedRepoRoot(context.Background(), projectID, repoName)
}

// SandboxRoot 解析 Run/API 使用的沙箱根目录（支持多仓 repository_id）。
func (s *Service) SandboxRoot(ctx context.Context, projectID uuid.UUID, repositoryID *uuid.UUID) (string, error) {
	if repositoryID != nil && s.repos != nil {
		r, err := s.repos.Get(ctx, *repositoryID)
		if err != nil {
			return "", err
		}
		if r.ProjectID != projectID {
			return "", fmt.Errorf("repository does not belong to project")
		}
		return s.namedRepoRoot(ctx, projectID, r.Name)
	}
	if s.repos != nil {
		if r, err := s.repos.GetDefault(ctx, projectID); err == nil {
			return s.namedRepoRoot(ctx, projectID, r.Name)
		}
	}
	return s.namedRepoRoot(ctx, projectID, "default")
}

// namedRepoRoot 返回指定名称仓库的根目录。
func (s *Service) namedRepoRoot(ctx context.Context, projectID uuid.UUID, repoName string) (string, error) {
	if repoName == "" {
		repoName = "default"
	}
	key, err := s.ProjectWorkspaceKey(ctx, projectID)
	if err != nil {
		return "", err
	}
	return storage.ProjectNamedRepoDir(s.paths, key, repoName), nil
}

// EnsureClone 确保默认仓库已克隆。
func (s *Service) EnsureClone(ctx context.Context, projectID uuid.UUID, gitURL, branch string) error {
	return s.EnsureRepo(ctx, projectID, "default", gitURL, branch)
}

// EnsureRepo 确保指定仓库已克隆。
func (s *Service) EnsureRepo(ctx context.Context, projectID uuid.UUID, name, gitURL, branch string) error {
	root, err := s.namedRepoRoot(ctx, projectID, name)
	if err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(root, ".git")); err == nil {
		return s.PullNamed(ctx, projectID, name)
	}
	if gitURL == "" {
		return os.MkdirAll(root, 0o755)
	}
	if branch == "" {
		branch = "main"
	}
	if err := os.MkdirAll(filepath.Dir(root), 0o755); err != nil {
		return err
	}
	timeout := s.git.CloneTimeout
	if timeout <= 0 {
		timeout = 300 * time.Second
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(cctx, "git", "clone", "--depth", "1", "-b", branch, gitURL, root)
	if env := gitutil.SSHCommandEnv(s.git, gitURL); env != nil {
		cmd.Env = env
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git clone: %w: %s", err, string(out))
	}
	return nil
}

// Pull 拉取 Git 仓库最新代码。
func (s *Service) Pull(ctx context.Context, projectID uuid.UUID) error {
	return s.PullNamed(ctx, projectID, "default")
}

// PullNamed 拉取指定名称的仓库。
func (s *Service) PullNamed(ctx context.Context, projectID uuid.UUID, name string) error {
	root, err := s.namedRepoRoot(ctx, projectID, name)
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, "git", "-C", root, "pull", "--ff-only")
	if env := gitutil.SSHCommandEnv(s.git, s.remoteURL(ctx, projectID, name)); env != nil {
		cmd.Env = env
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git pull: %w: %s", err, string(out))
	}
	return nil
}

// PullAll 拉取项目全部仓库。
func (s *Service) PullAll(ctx context.Context, projectID uuid.UUID) error {
	if s.repos == nil {
		return s.Pull(ctx, projectID)
	}
	repos, err := s.repos.List(ctx, projectID)
	if err != nil {
		return err
	}
	for _, r := range repos {
		if err := s.EnsureRepo(ctx, projectID, r.Name, r.GitURL, r.GitBranch); err != nil {
			return fmt.Errorf("repo %s: %w", r.Name, err)
		}
	}
	return nil
}

// PullByID 按仓库 ID 拉取最新代码。
func (s *Service) PullByID(ctx context.Context, projectID, repoID uuid.UUID) error {
	if s.repos == nil {
		return s.Pull(ctx, projectID)
	}
	r, err := s.repos.Get(ctx, repoID)
	if err != nil {
		return err
	}
	return s.EnsureRepo(ctx, projectID, r.Name, r.GitURL, r.GitBranch)
}

// Push 推送 Git 变更。
func (s *Service) Push(ctx context.Context, projectID uuid.UUID, message string) error {
	return s.PushNamed(ctx, projectID, "default", message)
}

// PushNamed 推送指定名称的仓库。
func (s *Service) PushNamed(ctx context.Context, projectID uuid.UUID, name, message string) error {
	root, err := s.namedRepoRoot(ctx, projectID, name)
	if err != nil {
		return err
	}
	if message == "" {
		message = "matrix: agent changes"
	}
	for _, args := range [][]string{
		{"add", "-A"},
		{"commit", "-m", message},
		{"push"},
	} {
		cmd := exec.CommandContext(ctx, "git", append([]string{"-C", root}, args...)...)
		if args[0] == "push" {
			if env := gitutil.SSHCommandEnv(s.git, s.remoteURL(ctx, projectID, name)); env != nil {
				cmd.Env = env
			}
		}
		out, err := cmd.CombinedOutput()
		if err != nil {
			if args[0] == "commit" && strings.Contains(string(out), "nothing to commit") {
				continue
			}
			return fmt.Errorf("git %s: %w: %s", args[0], err, string(out))
		}
	}
	return nil
}

// PushByID 按仓库 ID 推送变更。
func (s *Service) PushByID(ctx context.Context, projectID, repoID uuid.UUID, message string) error {
	if s.repos == nil {
		return s.Push(ctx, projectID, message)
	}
	r, err := s.repos.Get(ctx, repoID)
	if err != nil {
		return err
	}
	return s.PushNamed(ctx, projectID, r.Name, message)
}

// FileEntry 是工作区文件树节点 API 返回的数据传输对象。
type FileEntry struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	IsDir bool   `json:"is_dir"`
	Size  int64  `json:"size"`
}

// ListFiles 返回列表。
func (s *Service) ListFiles(projectID uuid.UUID, rel string) ([]FileEntry, error) {
	return s.ListFilesFor(projectID, "", rel)
}

// ListFilesFor 列出指定仓库目录下的文件。
func (s *Service) ListFilesFor(projectID uuid.UUID, repoName, rel string) ([]FileEntry, error) {
	full, err := s.resolveFor(projectID, repoName, rel)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(full)
	if err != nil {
		return nil, err
	}
	out := make([]FileEntry, 0, len(entries))
	for _, e := range entries {
		info, _ := e.Info()
		size := int64(0)
		if info != nil && !e.IsDir() {
			size = info.Size()
		}
		p := filepath.ToSlash(filepath.Join(rel, e.Name()))
		out = append(out, FileEntry{Name: e.Name(), Path: p, IsDir: e.IsDir(), Size: size})
	}
	return out, nil
}

// ReadFile 读取工作区文件内容。
func (s *Service) ReadFile(projectID uuid.UUID, rel string) (string, error) {
	return s.ReadFileFor(projectID, "", rel)
}

// ReadFileFor 读取指定仓库内的文件内容。
func (s *Service) ReadFileFor(projectID uuid.UUID, repoName, rel string) (string, error) {
	full, err := s.resolveFor(projectID, repoName, rel)
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(full)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// WriteFile 写入工作区文件。
func (s *Service) WriteFile(projectID uuid.UUID, rel, content string) error {
	return s.WriteFileFor(projectID, "", rel, content)
}

// WriteFileFor 写入指定仓库内的文件。
func (s *Service) WriteFileFor(projectID uuid.UUID, repoName, rel, content string) error {
	full, err := s.resolveFor(projectID, repoName, rel)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	return os.WriteFile(full, []byte(content), 0o644)
}

// resolve 解析。
func (s *Service) resolve(projectID uuid.UUID, rel string) (string, error) {
	return s.resolveFor(projectID, "", rel)
}

// resolveFor 解析For。
func (s *Service) resolveFor(projectID uuid.UUID, repoName, rel string) (string, error) {
	root, err := s.namedRepoRoot(context.Background(), projectID, repoName)
	if err != nil {
		return "", err
	}
	rel = strings.TrimPrefix(filepath.Clean(rel), "/")
	if rel == "." {
		return root, nil
	}
	full := filepath.Clean(filepath.Join(root, rel))
	if !strings.HasPrefix(full, filepath.Clean(root)) {
		return "", fmt.Errorf("path escapes workspace")
	}
	return full, nil
}

// Status 返回 Git 工作区状态。
func (s *Service) Status(ctx context.Context, projectID uuid.UUID) (string, error) {
	root, err := s.namedRepoRoot(context.Background(), projectID, "default")
	if err != nil {
		return "", err
	}
	cmd := exec.CommandContext(ctx, "git", "-C", root, "status", "--short")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// ReadSeek 按偏移读取文件片段。
func (s *Service) ReadSeek(projectID uuid.UUID, rel string, max int) ([]byte, error) {
	full, err := s.resolve(projectID, rel)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(full)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	buf := make([]byte, max)
	n, err := f.Read(buf)
	if err != nil && n == 0 {
		return nil, err
	}
	return buf[:n], nil
}

// remoteURL 解析项目或仓库的 Git 远程 URL。
func (s *Service) remoteURL(ctx context.Context, projectID uuid.UUID, name string) string {
	if name == "" {
		name = "default"
	}
	if s.repos != nil {
		list, err := s.repos.List(ctx, projectID)
		if err == nil {
			for _, r := range list {
				if r.Name == name {
					return r.GitURL
				}
			}
			if name == "default" {
				if r, err := s.repos.GetDefault(ctx, projectID); err == nil {
					return r.GitURL
				}
			}
		}
	}
	root, err := s.namedRepoRoot(ctx, projectID, name)
	if err != nil {
		return ""
	}
	cmd := exec.CommandContext(ctx, "git", "-C", root, "remote", "get-url", "origin")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

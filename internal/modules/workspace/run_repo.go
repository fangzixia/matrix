package workspace

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"matrix/internal/platform/gitutil"
	"matrix/internal/platform/storage"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

// CopyRunRepoFrom 将实现 Run 的 repo 目录复制到目标 Run 沙箱（runs/{runID}/repo）。
func (s *Service) CopyRunRepoFrom(ctx context.Context, projectID uuid.UUID, sourceRepoDir string, runID uuid.UUID) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	sourceRepoDir = filepath.Clean(strings.TrimSpace(sourceRepoDir))
	if sourceRepoDir == "" {
		return "", fmt.Errorf("source repo dir is empty")
	}
	info, err := os.Stat(sourceRepoDir)
	if err != nil {
		return "", fmt.Errorf("source repo: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("source repo is not a directory: %s", sourceRepoDir)
	}
	key, err := s.ProjectWorkspaceKey(ctx, projectID)
	if err != nil {
		return "", err
	}
	sandboxDir := storage.RunSandboxDir(s.paths, key, runID.String())
	repoDir := storage.RunRepoDir(s.paths, key, runID.String())
	if err := os.RemoveAll(sandboxDir); err != nil {
		return "", fmt.Errorf("prepare run sandbox: %w", err)
	}
	if err := copyDirTree(sourceRepoDir, repoDir); err != nil {
		_ = os.RemoveAll(sandboxDir)
		return "", fmt.Errorf("copy run repo: %w", err)
	}
	return repoDir, nil
}

func copyDirTree(src, dst string) error {
	src = filepath.Clean(src)
	dst = filepath.Clean(dst)
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !srcInfo.IsDir() {
		return fmt.Errorf("copy source is not a directory: %s", src)
	}
	if err := os.MkdirAll(dst, srcInfo.Mode().Perm()); err != nil {
		return err
	}
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			fi, err := d.Info()
			if err != nil {
				return err
			}
			return os.MkdirAll(target, fi.Mode().Perm())
		}
		fi, err := d.Info()
		if err != nil {
			return err
		}
		mode := fi.Mode()
		if mode&os.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			return os.Symlink(link, target)
		}
		return copyFile(path, target, mode)
	})
}

func copyFile(src, dst string, mode fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode.Perm())
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

// CreateRunRepo 为 Run 独立克隆 Git 仓库到 workspaces/{key}/runs/{runID}/repo。
func (s *Service) CreateRunRepo(ctx context.Context, projectID uuid.UUID, runID uuid.UUID, gitURL, branch string) (string, error) {
	key, err := s.ProjectWorkspaceKey(ctx, projectID)
	if err != nil {
		return "", err
	}
	sandboxDir := storage.RunSandboxDir(s.paths, key, runID.String())
	repoDir := storage.RunRepoDir(s.paths, key, runID.String())
	if err := os.RemoveAll(sandboxDir); err != nil {
		return "", fmt.Errorf("prepare run sandbox: %w", err)
	}
	if gitURL == "" {
		if err := os.MkdirAll(repoDir, 0o755); err != nil {
			return "", err
		}
		return repoDir, nil
	}
	if branch == "" {
		branch = "main"
	}
	if err := os.MkdirAll(filepath.Dir(repoDir), 0o755); err != nil {
		return "", err
	}
	timeout := s.git.CloneTimeout
	if timeout <= 0 {
		timeout = 300 * time.Second
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(cctx, "git", "clone", "--depth", "1", "-b", branch, gitURL, repoDir)
	if env := gitutil.SSHCommandEnv(s.git, gitURL); env != nil {
		cmd.Env = env
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		_ = os.RemoveAll(sandboxDir)
		return "", fmt.Errorf("git clone: %w: %s", err, string(out))
	}
	return repoDir, nil
}

// RemoveRunRepo 删除 Run 沙箱目录。
func (s *Service) RemoveRunRepo(ctx context.Context, projectID uuid.UUID, runID uuid.UUID) error {
	key, err := s.ProjectWorkspaceKey(ctx, projectID)
	if err != nil {
		return err
	}
	return os.RemoveAll(storage.RunSandboxDir(s.paths, key, runID.String()))
}

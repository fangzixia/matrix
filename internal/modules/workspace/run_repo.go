package workspace

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"matrix/internal/platform/config"
	"matrix/internal/platform/gitutil"
	"matrix/internal/platform/logging"
	"matrix/internal/platform/storage"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	gitCloneTimeout  = 2 * time.Minute
	gitCloneAttempts = 3
)

// CopyRepo 将来源 repo 目录复制到目标 Run 沙箱（runs/{runID}/repo），避免重新 git clone。
func (s *Service) CopyRepo(ctx context.Context, projectID uuid.UUID, sourceRepoDir string, runID uuid.UUID) (string, error) {
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
	var lastErr error
	for attempt := 1; attempt <= gitCloneAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		if attempt > 1 {
			if err := os.RemoveAll(sandboxDir); err != nil {
				return "", fmt.Errorf("prepare run sandbox: %w", err)
			}
		}
		if err := os.MkdirAll(filepath.Dir(repoDir), 0o755); err != nil {
			return "", err
		}
		logging.Info("workspace: git clone 开始",
			"run_id", runID, "attempt", attempt, "max_attempts", gitCloneAttempts,
			"branch", branch, "git_url", gitURL,
		)
		gitCfg, err := s.settings.LoadGitConfig(ctx)
		if err != nil {
			return "", err
		}
		_, err = runGitClone(ctx, gitURL, branch, repoDir, gitCfg)
		if err == nil {
			logging.Info("workspace: git clone 成功", "run_id", runID, "attempt", attempt)
			return repoDir, nil
		}
		lastErr = err
		logging.Warn("workspace: git clone 失败",
			"run_id", runID, "attempt", attempt, "error", err.Error(),
		)
		_ = os.RemoveAll(sandboxDir)
	}
	return "", &SourceFetchError{Attempts: gitCloneAttempts, Cause: lastErr}
}

func runGitClone(ctx context.Context, gitURL, branch, repoDir string, gitCfg config.GitConfig) ([]byte, error) {
	cctx, cancel := context.WithTimeout(ctx, gitCloneTimeout)
	defer cancel()

	cmd := exec.CommandContext(cctx, "git", "clone", "--depth", "1", "-b", branch, gitURL, repoDir)
	if env := gitutil.SSHCommandEnv(gitCfg, gitURL); env != nil {
		cmd.Env = env
	}
	return runGitCloneCmd(cctx, cmd)
}

// runGitCloneCmd 执行 git clone；超时时终止进程树（Windows 上 git 会 spawn index-pack 子进程）。
func runGitCloneCmd(ctx context.Context, cmd *exec.Cmd) ([]byte, error) {
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Start(); err != nil {
		return buf.Bytes(), err
	}
	waitDone := make(chan error, 1)
	go func() { waitDone <- cmd.Wait() }()

	select {
	case err := <-waitDone:
		if err != nil {
			return buf.Bytes(), fmt.Errorf("%w: %s", err, strings.TrimSpace(buf.String()))
		}
		return buf.Bytes(), nil
	case <-ctx.Done():
		if cmd.Process != nil {
			killProcessTree(cmd.Process.Pid)
		}
		<-waitDone
		msg := strings.TrimSpace(buf.String())
		if msg != "" {
			return buf.Bytes(), fmt.Errorf("git clone 超时: %s", msg)
		}
		return buf.Bytes(), fmt.Errorf("git clone 超时: %w", ctx.Err())
	}
}

func killProcessTree(pid int) {
	if pid <= 0 {
		return
	}
	if runtime.GOOS == "windows" {
		_ = exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(pid)).Run()
		return
	}
	_ = exec.Command("kill", "-TERM", strconv.Itoa(pid)).Run()
}

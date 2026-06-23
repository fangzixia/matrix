package workspace

import (
	"context"
	"fmt"
	"matrix/internal/platform/storage"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

// MergeResult 是 worktree 合并结果。
type MergeResult struct {
	Conflicts []string
}

// CreateRunWorktree 基于主仓库创建 Run 专用 worktree；无 Git 时仅创建空目录。
func (s *Service) CreateRunWorktree(ctx context.Context, projectID uuid.UUID, repoName string, runID uuid.UUID) (sandboxPath, branch string, err error) {
	if repoName == "" {
		repoName = "default"
	}
	key, err := s.ProjectWorkspaceKey(ctx, projectID)
	if err != nil {
		return "", "", err
	}
	mainRoot, err := s.namedRepoRoot(ctx, projectID, repoName)
	if err != nil {
		return "", "", err
	}
	wtPath := storage.RunWorktreeDir(s.paths, key, runID.String())
	if err := os.MkdirAll(filepath.Dir(wtPath), 0o755); err != nil {
		return "", "", err
	}
	if _, err := os.Stat(filepath.Join(mainRoot, ".git")); err != nil {
		if err := os.MkdirAll(wtPath, 0o755); err != nil {
			return "", "", err
		}
		return wtPath, "", nil
	}
	if err := s.PullNamed(ctx, projectID, repoName); err != nil {
		return "", "", fmt.Errorf("pull before worktree: %w", err)
	}
	if st, err := os.Stat(wtPath); err == nil && st.IsDir() {
		return wtPath, runBranchName(runID), nil
	}
	branch = runBranchName(runID)
	cmd := execCommand(ctx, mainRoot, "worktree", "add", "-b", branch, wtPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		if strings.Contains(string(out), "already exists") {
			_ = s.RemoveRunWorktree(ctx, projectID, repoName, runID, branch, wtPath)
			cmd = execCommand(ctx, mainRoot, "worktree", "add", "-b", branch, wtPath)
			out, err = cmd.CombinedOutput()
		}
		if err != nil {
			return "", "", fmt.Errorf("git worktree add: %w: %s", err, string(out))
		}
	}
	return wtPath, branch, nil
}

// RemoveRunWorktree 删除 Run worktree 及对应分支。
func (s *Service) RemoveRunWorktree(ctx context.Context, projectID uuid.UUID, repoName string, runID uuid.UUID, branch, wtPath string) error {
	if repoName == "" {
		repoName = "default"
	}
	key, err := s.ProjectWorkspaceKey(ctx, projectID)
	if err != nil {
		return err
	}
	if wtPath == "" {
		wtPath = storage.RunWorktreeDir(s.paths, key, runID.String())
	}
	mainRoot, err := s.namedRepoRoot(ctx, projectID, repoName)
	if err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(mainRoot, ".git")); err != nil {
		return os.RemoveAll(wtPath)
	}
	if branch == "" {
		branch = runBranchName(runID)
	}
	if _, err := os.Stat(wtPath); err == nil {
		cmd := execCommand(ctx, mainRoot, "worktree", "remove", "--force", wtPath)
		if out, err := cmd.CombinedOutput(); err != nil && !strings.Contains(string(out), "is not a working tree") {
			_ = os.RemoveAll(wtPath)
		}
	}
	delCmd := execCommand(ctx, mainRoot, "branch", "-D", branch)
	_, _ = delCmd.CombinedOutput()
	prune := execCommand(ctx, mainRoot, "worktree", "prune")
	_, _ = prune.CombinedOutput()
	return nil
}

// MergeRunWorktree 将 Run worktree 分支合并回主仓库。
func (s *Service) MergeRunWorktree(ctx context.Context, projectID uuid.UUID, repoName string, runID uuid.UUID, branch, wtPath string) (*MergeResult, error) {
	if repoName == "" {
		repoName = "default"
	}
	key, err := s.ProjectWorkspaceKey(ctx, projectID)
	if err != nil {
		return nil, err
	}
	mainRoot, err := s.namedRepoRoot(ctx, projectID, repoName)
	if err != nil {
		return nil, err
	}
	if wtPath == "" {
		wtPath = storage.RunWorktreeDir(s.paths, key, runID.String())
	}
	if branch == "" {
		branch = runBranchName(runID)
	}
	if _, err := os.Stat(filepath.Join(mainRoot, ".git")); err != nil {
		return nil, fmt.Errorf("主仓库不是 Git 仓库，无法合并")
	}
	for _, args := range [][]string{
		{"add", "-A"},
		{"commit", "-m", fmt.Sprintf("matrix: run %s", runID.String())},
	} {
		cmd := execCommand(ctx, wtPath, args...)
		out, err := cmd.CombinedOutput()
		if err != nil && args[0] == "commit" && strings.Contains(string(out), "nothing to commit") {
			continue
		}
		if err != nil && args[0] != "commit" {
			return nil, fmt.Errorf("git %s in worktree: %w: %s", args[0], err, string(out))
		}
	}
	mergeCmd := execCommand(ctx, mainRoot, "merge", branch, "--no-edit")
	out, err := mergeCmd.CombinedOutput()
	if err != nil {
		conflicts := parseConflictFiles(string(out), mainRoot)
		if len(conflicts) == 0 {
			conflicts = listUnmerged(mainRoot)
		}
		abort := execCommand(ctx, mainRoot, "merge", "--abort")
		_, _ = abort.CombinedOutput()
		return &MergeResult{Conflicts: conflicts}, fmt.Errorf("merge conflict")
	}
	return &MergeResult{}, nil
}

// runBranchName 生成 Run 专用 Git 分支名。
func runBranchName(runID uuid.UUID) string {
	id := strings.ReplaceAll(runID.String(), "-", "")
	if len(id) > 8 {
		id = id[:8]
	}
	return "matrix/run-" + id
}

// parseConflictFiles 从 git 输出解析冲突文件列表。
func parseConflictFiles(output, mainRoot string) []string {
	var conflicts []string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "CONFLICT") {
			if idx := strings.Index(line, "Merge conflict in "); idx >= 0 {
				conflicts = append(conflicts, strings.TrimSpace(line[idx+len("Merge conflict in "):]))
			}
		}
	}
	if len(conflicts) == 0 {
		conflicts = listUnmerged(mainRoot)
	}
	return conflicts
}

// listUnmerged 列出 worktree 中未合并的文件。
func listUnmerged(mainRoot string) []string {
	cmd := execCommand(context.Background(), mainRoot, "diff", "--name-only", "--diff-filter=U")
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	var files []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			files = append(files, line)
		}
	}
	return files
}

// execCommand 在指定目录执行 shell 命令。
func execCommand(ctx context.Context, dir string, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
}

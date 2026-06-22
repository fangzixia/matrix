// Package storage 解析配置中的本地目录路径并确保目录布局存在。
package storage

import (
	"fmt"

	"os"

	"path/filepath"

	"strings"

	"matrix/internal/platform/config"
)

// Paths 是解析后的本地存储目录绝对路径集合。
type Paths struct {
	DataDir string

	WorkspacesDir string

	AuditDir string

	ExportsDir string

	LogDir string

	LogFile string
}

// Resolve 根据配置计算各数据目录的绝对路径并校验 allowed_roots。
func Resolve(cfg *config.Config) (Paths, error) {

	base := config.ResolvePath(".", cfg.Storage.BaseDir)

	data := config.ResolvePath(base, cfg.Storage.DataDir)

	p := Paths{

		DataDir: data,

		WorkspacesDir: config.ResolvePath(data, cfg.Storage.WorkspacesDir),

		AuditDir: config.ResolvePath(data, cfg.Storage.AuditDir),

		ExportsDir: config.ResolvePath(data, cfg.Storage.ExportsDir),

		LogDir: config.ResolvePath(".", cfg.Logging.Dir),

		LogFile: filepath.Join(config.ResolvePath(".", cfg.Logging.Dir), cfg.Logging.File),
	}

	if err := validateAllowedRoots(cfg.Storage.AllowedRoots, p); err != nil {

		return Paths{}, err

	}

	return p, nil

}

func validateAllowedRoots(allowed []string, p Paths) error {

	if len(allowed) == 0 {

		return nil

	}

	paths := []string{p.DataDir, p.WorkspacesDir, p.AuditDir, p.ExportsDir, p.LogDir}

	for _, target := range paths {

		abs, err := filepath.Abs(target)

		if err != nil {

			return fmt.Errorf("storage: resolve %s: %w", target, err)

		}

		if !underAllowedRoot(abs, allowed) {

			return fmt.Errorf("storage: path %s not under allowed_roots", abs)

		}

	}

	return nil

}

func underAllowedRoot(path string, allowed []string) bool {

	path = filepath.Clean(path)

	for _, root := range allowed {

		root = strings.TrimSpace(root)

		if root == "" {

			continue

		}

		abs, err := filepath.Abs(root)

		if err != nil {

			continue

		}

		abs = filepath.Clean(abs)

		if path == abs || strings.HasPrefix(path, abs+string(os.PathSeparator)) {

			return true

		}

	}

	return false

}

// EnsureLayout 创建 DataDir、WorkspacesDir 等必要目录。
func EnsureLayout(p Paths) error {

	for _, dir := range []string{p.DataDir, p.WorkspacesDir, p.AuditDir, p.ExportsDir, p.LogDir} {

		if err := os.MkdirAll(dir, 0o755); err != nil {

			return fmt.Errorf("storage: mkdir %s: %w", dir, err)

		}

	}

	return nil

}

// ProjectRepoDir 返回项目默认 Git 仓库克隆目录。
func ProjectRepoDir(p Paths, projectID string) string {
	return filepath.Join(p.WorkspacesDir, projectID, "repo")
}

// ProjectNamedRepoDir 返回项目具名 Git 仓库克隆目录。
func ProjectNamedRepoDir(p Paths, projectID, repoName string) string {
	if repoName == "" || repoName == "default" {
		return ProjectRepoDir(p, projectID)
	}
	return filepath.Join(p.WorkspacesDir, projectID, "repos", repoName)
}

// ProjectAuditDir 返回项目审计日志根目录。
func ProjectAuditDir(p Paths, projectID string) string {

	return filepath.Join(p.AuditDir, projectID)

}

// ProjectSessionsDir 返回项目 Chat/Agent 会话 transcript 目录。
func ProjectSessionsDir(p Paths, projectID string) string {

	return filepath.Join(ProjectAuditDir(p, projectID), "sessions")

}

// RunAuditFile 返回单次 Run 的 JSONL 审计文件路径。
func RunAuditFile(p Paths, projectID, runID string) string {

	return filepath.Join(ProjectAuditDir(p, projectID), runID+".jsonl")

}

// ProjectSubagentsDir 返回项目子 Agent sidechain transcript 目录。
func ProjectSubagentsDir(sessionsDir string) string {

	return filepath.Join(filepath.Dir(sessionsDir), "subagents")

}

// RunWorktreeDir 返回单次 Run 的 Git worktree 工作目录。
func RunWorktreeDir(p Paths, projectID, runID string) string {
	return filepath.Join(p.WorkspacesDir, projectID, "runs", runID)
}

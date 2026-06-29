// Package storage 解析配置中的本地目录路径并确保目录布局存在。
package storage

import (
	"fmt"
	"matrix/internal/platform/config"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Paths 是解析后的本地存储目录绝对路径集合。
type Paths struct {
	DataDir       string
	WorkspacesDir string
	AuditDir      string
	ExportsDir    string
	LogDir        string
	LogCategories LogCategories
}

// LogCategories 是各类别日志子目录名（实际文件为 {dir}/{category}/{YYYY-MM-DD}.log）。
type LogCategories struct {
	System string
	API    string
	LLM    string
	Agent  string
}

// LogPath 返回指定类别与日期的日志文件绝对路径。
func (p Paths) LogPath(category string, date time.Time) string {
	return filepath.Join(p.LogDir, category, date.Format("2006-01-02")+".log")
}

// Resolve 根据配置计算各数据目录的绝对路径并校验 allowed_roots。
func Resolve(cfg *config.Config) (Paths, error) {
	base := config.ResolvePath(".", cfg.Storage.BaseDir)
	data := config.ResolvePath(base, cfg.Storage.DataDir)
	p := Paths{
		DataDir:       data,
		WorkspacesDir: config.ResolvePath(data, cfg.Storage.WorkspacesDir),
		AuditDir:      config.ResolvePath(data, cfg.Storage.AuditDir),
		ExportsDir:    config.ResolvePath(data, cfg.Storage.ExportsDir),
		LogDir:        config.ResolvePath(".", cfg.Logging.Dir),
		LogCategories: LogCategories{
			System: cfg.Logging.Categories.System,
			API:    cfg.Logging.Categories.API,
			LLM:    cfg.Logging.Categories.LLM,
			Agent:  cfg.Logging.Categories.Agent,
		},
	}
	if err := validateAllowedRoots(cfg.Storage.AllowedRoots, p); err != nil {
		return Paths{}, err
	}
	return p, nil
}

// validateAllowedRoots 校验存储路径是否在允许根目录下。
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

// underAllowedRoot 判断路径是否位于允许的根目录内。
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
	dirs := []string{p.DataDir, p.WorkspacesDir, p.AuditDir, p.ExportsDir, p.LogDir}
	for _, cat := range []string{p.LogCategories.System, p.LogCategories.API, p.LogCategories.LLM, p.LogCategories.Agent} {
		if cat != "" {
			dirs = append(dirs, filepath.Join(p.LogDir, cat))
		}
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("storage: mkdir %s: %w", dir, err)
		}
	}
	return nil
}

// ProjectRepoDir 返回项目默认 Git 仓库克隆目录。
func ProjectRepoDir(p Paths, projectKey string) string {
	return filepath.Join(p.WorkspacesDir, projectKey, "repo")
}

// ProjectNamedRepoDir 返回项目具名 Git 仓库克隆目录。
func ProjectNamedRepoDir(p Paths, projectKey, repoName string) string {
	if repoName == "" || repoName == "default" {
		return ProjectRepoDir(p, projectKey)
	}
	return filepath.Join(p.WorkspacesDir, projectKey, "repos", repoName)
}

// ProjectAuditDir 返回项目审计日志根目录。
func ProjectAuditDir(p Paths, projectKey string) string {
	return filepath.Join(p.AuditDir, projectKey)
}

// ProjectSessionsDir 返回项目 Chat/Agent 会话 transcript 目录。
func ProjectSessionsDir(p Paths, projectKey string) string {
	return filepath.Join(ProjectAuditDir(p, projectKey), "sessions")
}

// RunAuditFile 返回单次 Run 的 JSONL 审计文件路径。
func RunAuditFile(p Paths, projectKey, runID string) string {
	return filepath.Join(ProjectAuditDir(p, projectKey), runID+".jsonl")
}

// ProjectSubagentsDir 返回项目子 Agent sidechain transcript 目录。
func ProjectSubagentsDir(sessionsDir string) string {
	return filepath.Join(filepath.Dir(sessionsDir), "subagents")
}

// RunWorktreeDir 返回单次 Run 的 Git worktree 工作目录。
func RunWorktreeDir(p Paths, projectKey, runID string) string {
	return filepath.Join(p.WorkspacesDir, projectKey, "runs", runID)
}

// ProjectDocsDir 返回项目计划/评测文档根目录（独立于 Git 源码仓库）。
func ProjectDocsDir(p Paths, projectKey string) string {
	return filepath.Join(p.WorkspacesDir, projectKey, "docs")
}

// ProjectDocsPlansDir 返回计划文档目录。
func ProjectDocsPlansDir(p Paths, projectKey string) string {
	return filepath.Join(ProjectDocsDir(p, projectKey), "plans")
}

// ProjectDocsEvaluationsDir 返回评测报告目录。
func ProjectDocsEvaluationsDir(p Paths, projectKey string) string {
	return filepath.Join(ProjectDocsDir(p, projectKey), "evaluations")
}

// ProjectMatrixDir 返回 Agent 运行时数据根目录（独立于 Git 源码仓库）。
func ProjectMatrixDir(p Paths, projectKey string) string {
	return filepath.Join(p.WorkspacesDir, projectKey, ".matrix")
}

// ProjectMatrixRunDir 返回单次 Run 的 Agent 运行时数据目录。
func ProjectMatrixRunDir(p Paths, projectKey, runID string) string {
	return filepath.Join(ProjectMatrixDir(p, projectKey), "runs", runID)
}

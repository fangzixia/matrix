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
}

// 业务数据子目录名（硬编码，相对 DataDir）。
const (
	dirWorkspaces = "workspaces"
	dirAudit      = "audit"
	dirExports    = "exports"
)

// 日志子目录名（硬编码，对应 logs/{category}/{YYYY-MM-DD}.log）。
const (
	LogCategorySystem = "system"
	LogCategoryAPI    = "api"
	LogCategoryLLM    = "llm"
	LogCategoryAgent  = "agent"
)

// LogCategoryDirs 返回全部日志类别子目录名。
func LogCategoryDirs() []string {
	return []string{LogCategorySystem, LogCategoryAPI, LogCategoryLLM, LogCategoryAgent}
}

// LogPath 返回指定类别与日期的日志文件绝对路径。
func (p Paths) LogPath(category string, date time.Time) string {
	return filepath.Join(p.LogDir, category, date.Format("2006-01-02")+".log")
}

// Resolve 根据配置计算各数据目录的绝对路径并校验 allowed_roots。
func Resolve(cfg *config.Config) (Paths, error) {
	data := config.ResolvePath(".", cfg.Storage.DataDir)
	p := Paths{
		DataDir:       data,
		WorkspacesDir: filepath.Join(data, dirWorkspaces),
		AuditDir:      filepath.Join(data, dirAudit),
		ExportsDir:    filepath.Join(data, dirExports),
		LogDir:        config.ResolvePath(".", cfg.Logging.Dir),
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
	for _, cat := range LogCategoryDirs() {
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

// RunSandboxDir 返回单次 Run 的沙箱根目录（含 repo 子目录）。
func RunSandboxDir(p Paths, projectKey, runID string) string {
	return filepath.Join(p.WorkspacesDir, projectKey, "runs", runID)
}

// RunRepoDir 返回单次 Run 的独立 Git 仓库克隆目录。
func RunRepoDir(p Paths, projectKey, runID string) string {
	return filepath.Join(RunSandboxDir(p, projectKey, runID), "repo")
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

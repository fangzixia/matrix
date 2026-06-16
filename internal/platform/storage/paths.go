package storage

import (
	"fmt"

	"os"

	"path/filepath"

	"strings"

	"matrix/internal/platform/config"
)

type Paths struct {
	DataDir string

	WorkspacesDir string

	AuditDir string

	ExportsDir string

	LogDir string

	LogFile string
}

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

func EnsureLayout(p Paths) error {

	for _, dir := range []string{p.DataDir, p.WorkspacesDir, p.AuditDir, p.ExportsDir, p.LogDir} {

		if err := os.MkdirAll(dir, 0o755); err != nil {

			return fmt.Errorf("storage: mkdir %s: %w", dir, err)

		}

	}

	return nil

}

func ProjectRepoDir(p Paths, projectID string) string {
	return filepath.Join(p.WorkspacesDir, projectID, "repo")
}

func ProjectNamedRepoDir(p Paths, projectID, repoName string) string {
	if repoName == "" || repoName == "default" {
		return ProjectRepoDir(p, projectID)
	}
	return filepath.Join(p.WorkspacesDir, projectID, "repos", repoName)
}

func ProjectAuditDir(p Paths, projectID string) string {

	return filepath.Join(p.AuditDir, projectID)

}

func ProjectSessionsDir(p Paths, projectID string) string {

	return filepath.Join(ProjectAuditDir(p, projectID), "sessions")

}

func RunAuditFile(p Paths, projectID, runID string) string {

	return filepath.Join(ProjectAuditDir(p, projectID), runID+".jsonl")

}

func ProjectSubagentsDir(sessionsDir string) string {

	return filepath.Join(filepath.Dir(sessionsDir), "subagents")

}

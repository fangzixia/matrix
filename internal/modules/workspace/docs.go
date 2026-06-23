package workspace

import (
	"context"
	"fmt"
	"matrix/internal/platform/pathutil"
	"matrix/internal/platform/storage"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

const (
	// DocsDirRel 是文档根目录的逻辑路径前缀。
	DocsDirRel = "docs"
	// DocsPlansRel 是计划文档的逻辑路径前缀。
	DocsPlansRel = "docs/plans"
	// DocsEvaluationsRel 是评测报告的逻辑路径前缀。
	DocsEvaluationsRel = "docs/evaluations"
)

// DocsRoot 返回项目文档根目录绝对路径（{workspaces}/{key}/docs）。
func (s *Service) DocsRoot(projectID uuid.UUID) (string, error) {
	return s.docsRoot(context.Background(), projectID)
}

// docsRoot 返回项目文档根目录绝对路径。
func (s *Service) docsRoot(ctx context.Context, projectID uuid.UUID) (string, error) {
	key, err := s.ProjectWorkspaceKey(ctx, projectID)
	if err != nil {
		return "", err
	}
	return storage.ProjectDocsDir(s.paths, key), nil
}

// EnsureDocsLayout 创建 docs/plans 与 docs/evaluations 目录。
func (s *Service) EnsureDocsLayout(projectID uuid.UUID) error {
	key, err := s.ProjectWorkspaceKey(context.Background(), projectID)
	if err != nil {
		return err
	}
	for _, dir := range []string{
		storage.ProjectDocsDir(s.paths, key),
		storage.ProjectDocsPlansDir(s.paths, key),
		storage.ProjectDocsEvaluationsDir(s.paths, key),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("workspace: mkdir docs %s: %w", dir, err)
		}
	}
	return nil
}

// SanitizeDocLogicalPath 校验文档逻辑路径；空路径合法，非空必须以 docs/ 开头。
func SanitizeDocLogicalPath(path string) (string, error) {
	path = filepath.ToSlash(strings.TrimSpace(path))
	if path == "" {
		return "", nil
	}
	if !strings.HasPrefix(path, DocsDirRel+"/") {
		return "", fmt.Errorf("doc path must be under %s/", DocsDirRel)
	}
	return path, nil
}

// ResolveDocPath 将逻辑路径（docs/plans/…）解析为绝对路径并校验不越界。
func (s *Service) ResolveDocPath(projectID uuid.UUID, logicalPath string) (string, error) {
	logicalPath, err := SanitizeDocLogicalPath(logicalPath)
	if err != nil {
		return "", err
	}
	if logicalPath == "" {
		return "", fmt.Errorf("empty doc path")
	}
	docsRoot, err := s.DocsRoot(projectID)
	if err != nil {
		return "", err
	}
	docsRoot = filepath.Clean(docsRoot)
	inner := strings.TrimPrefix(logicalPath, DocsDirRel+"/")
	full := filepath.Clean(filepath.Join(docsRoot, filepath.FromSlash(inner)))
	if !pathutil.WithinRoot(full, docsRoot) {
		return "", fmt.Errorf("path escapes docs directory")
	}
	return full, nil
}

// DocSandboxDir 返回供 AI 沙箱额外挂载的文档根目录。
func (s *Service) DocSandboxDir(projectID uuid.UUID) (string, error) {
	return s.DocsRoot(projectID)
}

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

// RunRepoRoot 返回单次 Run 的 Git 仓库根目录（workspaces/{key}/runs/{runID}/repo）。
func (s *Service) RunRepoRoot(ctx context.Context, projectID, runID uuid.UUID) (string, error) {
	key, err := s.ProjectWorkspaceKey(ctx, projectID)
	if err != nil {
		return "", err
	}
	return storage.RunRepoDir(s.paths, key, runID.String()), nil
}

// ListRunFiles 列出 Run 仓库目录下的文件。
func (s *Service) ListRunFiles(ctx context.Context, projectID, runID uuid.UUID, rel string) ([]FileEntry, error) {
	full, err := s.resolveRunRepo(projectID, runID, rel)
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

// ReadRunFile 读取 Run 仓库内的文件内容。
func (s *Service) ReadRunFile(ctx context.Context, projectID, runID uuid.UUID, rel string) (string, error) {
	full, err := s.resolveRunRepo(projectID, runID, rel)
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(full)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (s *Service) resolveRunRepo(projectID, runID uuid.UUID, rel string) (string, error) {
	root, err := s.RunRepoRoot(context.Background(), projectID, runID)
	if err != nil {
		return "", err
	}
	rel = strings.TrimPrefix(filepath.Clean(rel), "/")
	if rel == "." {
		return root, nil
	}
	full := filepath.Clean(filepath.Join(root, rel))
	if !pathutil.WithinRoot(full, filepath.Clean(root)) {
		return "", fmt.Errorf("path escapes run repository")
	}
	return full, nil
}

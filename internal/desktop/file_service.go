package desktop

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FileService owns workspace-scoped file access for Wails-facing APIs.
type FileService struct {
	rootFn func() string
}

func NewFileService(rootFn func() string) *FileService {
	return &FileService{rootFn: rootFn}
}

func (s *FileService) root() string {
	if s == nil || s.rootFn == nil {
		return ""
	}
	return filepath.Clean(strings.TrimSpace(s.rootFn()))
}

// ResolveInWorkspace resolves path under the current workspace and rejects
// absolute or relative paths that escape the workspace root.
func (s *FileService) ResolveInWorkspace(path string) (string, error) {
	root := s.root()
	if root == "" || root == "." {
		return "", fmt.Errorf("未配置工作区")
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return root, nil
	}
	var full string
	if filepath.IsAbs(path) {
		full = filepath.Clean(path)
	} else {
		full = filepath.Clean(filepath.Join(root, path))
	}
	if !pathWithinRoot(full, root) {
		return "", fmt.Errorf("路径必须位于工作区内: %s", path)
	}
	return full, nil
}

func (s *FileService) ReadFile(path string) (string, error) {
	fullPath, err := s.ResolveInWorkspace(path)
	if err != nil {
		return "", err
	}
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return "", fmt.Errorf("read file: %w", err)
	}
	return string(content), nil
}

func (s *FileService) SaveFile(path, content string) error {
	fullPath, err := s.ResolveInWorkspace(path)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}
	if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	return nil
}

func (s *FileService) ListFiles(path string) ([]FileInfo, error) {
	fullPath, err := s.ResolveInWorkspace(path)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(fullPath)
	if err != nil {
		return nil, fmt.Errorf("read directory: %w", err)
	}
	files := make([]FileInfo, 0, len(entries))
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		files = append(files, FileInfo{
			Name:  entry.Name(),
			IsDir: entry.IsDir(),
			Size:  info.Size(),
		})
	}
	return files, nil
}

func pathWithinRoot(absPath, root string) bool {
	absPath = filepath.Clean(absPath)
	root = filepath.Clean(root)
	if absPath == root {
		return true
	}
	rel, err := filepath.Rel(root, absPath)
	if err != nil || rel == ".." {
		return false
	}
	return !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

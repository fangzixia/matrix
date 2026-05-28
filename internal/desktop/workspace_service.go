package desktop

import (
	"os"
	"path/filepath"

	"matrix/internal/logger"
	"matrix/internal/matrixpaths"
	"matrix/internal/tools"
)

// WorkspaceService owns current workspace state and persistence.
type WorkspaceService struct {
	config    *Config
	onChanged func(oldRoot, newRoot string)
}

func NewWorkspaceService(cfg *Config, onChanged func(oldRoot, newRoot string)) *WorkspaceService {
	return &WorkspaceService{config: cfg, onChanged: onChanged}
}

func (s *WorkspaceService) Root() string {
	return matrixpaths.NormalizeWorkspacePath(s.config.Workspace.Root)
}

func (s *WorkspaceService) Get() map[string]interface{} {
	root := s.Root()
	out := map[string]interface{}{
		"current": root,
		"recent":  s.config.Workspace.Recent,
	}
	if root != "" && root != "." {
		out["workspaceId"] = matrixpaths.WorkspaceID(root)
	}
	return out
}

func (s *WorkspaceService) Set(path string) error {
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return NewAppError(ErrorWorkspace, "路径不存在或不是目录", false, err)
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		return NewAppError(ErrorValidation, "无效路径", false, err)
	}

	oldRoot := s.config.Workspace.Root
	if oldAbs, err := filepath.Abs(oldRoot); err == nil && oldRoot != "" {
		oldRoot = oldAbs
	}
	s.config.Workspace.Root = abs
	tools.SetWorkspaceRoot(matrixpaths.NormalizeWorkspacePath(abs))

	recent := []string{abs}
	for _, r := range s.config.Workspace.Recent {
		if r != abs && len(recent) < 10 {
			recent = append(recent, r)
		}
	}
	s.config.Workspace.Recent = recent

	if err := SaveConfig(s.config); err != nil {
		return wrapInternal("保存工作区配置失败", err)
	}
	if err := matrixpaths.EnsureWorkspaceStore(abs); err != nil {
		logger.Warnf("workspace store: %v", err)
	}
	if s.onChanged != nil && oldRoot != abs {
		s.onChanged(oldRoot, abs)
	}
	return nil
}

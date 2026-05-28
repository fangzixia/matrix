package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var (
	workspaceRootMu sync.RWMutex
	workspaceRoot   string // 绝对路径；空表示未配置工作区，文件工具退回进程 CWD
)

// SetWorkspaceRoot 设置当前会话的工作区根目录（由 desktop Bridge 在启动 Agent 时调用）。
func SetWorkspaceRoot(root string) {
	root = strings.TrimSpace(root)
	if root == "" || root == "." {
		workspaceRootMu.Lock()
		workspaceRoot = ""
		workspaceRootMu.Unlock()
		return
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		abs = root
	}
	workspaceRootMu.Lock()
	workspaceRoot = filepath.Clean(abs)
	workspaceRootMu.Unlock()
}

func getWorkspaceRoot() string {
	workspaceRootMu.RLock()
	defer workspaceRootMu.RUnlock()
	return workspaceRoot
}

// ResolveWorkspacePath 将工具路径解析为绝对路径：相对路径相对于工作区根，未配置时相对于进程 CWD。
func ResolveWorkspacePath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("路径为空")
	}
	if filepath.IsAbs(path) {
		return filepath.Clean(path), nil
	}
	base := getWorkspaceRoot()
	if base == "" {
		var err error
		base, err = os.Getwd()
		if err != nil {
			return "", fmt.Errorf("获取工作目录失败: %w", err)
		}
	}
	return filepath.Clean(filepath.Join(base, path)), nil
}

// RequirePathInWorkspace 校验目标路径必须位于已配置的工作区内。
func RequirePathInWorkspace(absPath string) error {
	root := getWorkspaceRoot()
	if root == "" {
		return nil
	}
	abs, err := filepath.Abs(absPath)
	if err != nil {
		return fmt.Errorf("无效路径: %w", err)
	}
	if !pathWithinRoot(abs, root) {
		return fmt.Errorf("文件操作必须位于工作区内 (%s)，拒绝访问: %s", root, abs)
	}
	return nil
}

func pathWithinRoot(absPath, root string) bool {
	absPath = filepath.Clean(absPath)
	root = filepath.Clean(root)
	if absPath == root {
		return true
	}
	rel, err := filepath.Rel(root, absPath)
	if err != nil {
		return false
	}
	if rel == ".." {
		return false
	}
	return !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

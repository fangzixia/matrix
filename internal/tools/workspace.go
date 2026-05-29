package tools

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"
)

var (
	workspaceRootMu sync.RWMutex
	workspaceRoot   string // 绝对路径；空表示未配置工作区
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

// pathParamDesc 为文件/目录类工具的 path 参数说明（必须显式传入绝对路径）。
const pathParamDesc = "必填。要访问的文件或目录的绝对路径（如 Windows: C:\\\\proj\\\\src）；不可使用相对路径。"

// readPathParamDesc 为 read_file 的 path 参数说明。
const readPathParamDesc = "必填。要读取文件的绝对路径；不可使用相对路径。"

// ErrToolPathRequired 表示调用方未在工具参数中提供 path。
var ErrToolPathRequired = fmt.Errorf("必须明确指定 path 参数，不可省略或使用 \".\"")

// ErrToolPathNotAbsolute 表示 path 不是绝对路径。
var ErrToolPathNotAbsolute = fmt.Errorf("path 必须为绝对路径，不可使用相对路径")

// RequireToolPath 校验 path 已由调用方显式提供。
func RequireToolPath(path string) error {
	p := strings.TrimSpace(path)
	if p == "" || p == "." {
		return ErrToolPathRequired
	}
	return nil
}

// ResolveAndValidateToolPath 校验 path 为绝对路径、规范化并确保位于工作区内。
func ResolveAndValidateToolPath(path string) (string, error) {
	if err := RequireToolPath(path); err != nil {
		return "", err
	}
	resolved, err := ResolveWorkspacePath(path)
	if err != nil {
		return "", err
	}
	if err := RequirePathInWorkspace(resolved); err != nil {
		return "", err
	}
	return resolved, nil
}

// FormatWorkerUserMessage 为 Worker 首条 user 消息附加工作区前缀（与 desktop Bridge 一致）。
func FormatWorkerUserMessage(msg string) string {
	msg = strings.TrimSpace(msg)
	root := getWorkspaceRoot()
	if root == "" || root == "." {
		return msg
	}
	if msg == "" {
		return fmt.Sprintf("工作区: %s", root)
	}
	return fmt.Sprintf("工作区: %s\n\n%s", root, msg)
}

// ResolveWorkspacePath 仅接受绝对路径，返回规范化后的绝对路径。
func ResolveWorkspacePath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("路径为空")
	}
	if !filepath.IsAbs(path) {
		return "", ErrToolPathNotAbsolute
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("无效路径: %w", err)
	}
	return filepath.Clean(abs), nil
}

// ToAbsolutePath 将路径转为绝对路径；若已是绝对路径则规范化，否则相对于 base 拼接（用于工具输出）。
func ToAbsolutePath(path, base string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return path
	}
	if filepath.IsAbs(path) {
		if abs, err := filepath.Abs(path); err == nil {
			return filepath.Clean(abs)
		}
		return filepath.Clean(path)
	}
	return filepath.Clean(filepath.Join(base, path))
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

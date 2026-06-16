package tools

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
)

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

// ResolveAndValidateToolPath 校验 path 为绝对路径、规范化并确保位于 context 沙箱内。
func ResolveAndValidateToolPath(ctx context.Context, path string) (string, error) {
	if err := RequireToolPath(path); err != nil {
		return "", err
	}
	resolved, err := ResolveWorkspacePath(path)
	if err != nil {
		return "", err
	}
	if err := RequirePathInSandbox(ctx, resolved); err != nil {
		return "", err
	}
	return resolved, nil
}

// FormatWorkerUserMessage 为 Worker 首条 user 消息附加沙箱路径前缀。
func FormatWorkerUserMessage(sandboxRoot, msg string) string {
	msg = strings.TrimSpace(msg)
	root := normalizeSandboxRoot(sandboxRoot)
	if root == "" {
		return msg
	}
	if msg == "" {
		return fmt.Sprintf("沙箱目录: %s", root)
	}
	return fmt.Sprintf("沙箱目录: %s\n\n%s", root, msg)
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

// RequirePathInSandbox 校验目标路径必须位于 context 绑定的沙箱目录内。
func RequirePathInSandbox(ctx context.Context, absPath string) error {
	root := SandboxFrom(ctx)
	if root == "" {
		return fmt.Errorf("沙箱未配置，拒绝文件访问")
	}
	abs, err := filepath.Abs(absPath)
	if err != nil {
		return fmt.Errorf("无效路径: %w", err)
	}
	if !pathWithinRoot(abs, root) {
		return fmt.Errorf("文件操作必须位于沙箱内 (%s)，拒绝访问: %s", root, abs)
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

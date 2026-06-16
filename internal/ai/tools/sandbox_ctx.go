package tools

import (
	"context"
	"path/filepath"
	"strings"
)

type sandboxKey struct{}

// WithSandbox 将项目沙箱根目录绑定到 context，供同一次 Run 及其子 Worker 共享。
func WithSandbox(ctx context.Context, root string) context.Context {
	root = normalizeSandboxRoot(root)
	if root == "" {
		return ctx
	}
	return context.WithValue(ctx, sandboxKey{}, root)
}

// SandboxFrom 返回当前 context 绑定的沙箱根目录（绝对路径）。
func SandboxFrom(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	root, _ := ctx.Value(sandboxKey{}).(string)
	return root
}

func normalizeSandboxRoot(root string) string {
	root = strings.TrimSpace(root)
	if root == "" || root == "." {
		return ""
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return filepath.Clean(root)
	}
	return filepath.Clean(abs)
}

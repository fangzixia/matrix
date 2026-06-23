package tools

import (
	"context"
	"path/filepath"
	"strings"
)

type sandboxKey struct{}

type extraSandboxKey struct{}

// WithSandbox 将项目沙箱根目录绑定到 context，供同一次 Run 及其子 Worker 共享。
func WithSandbox(ctx context.Context, root string) context.Context {
	root = normalizeSandboxRoot(root)
	if root == "" {
		return ctx
	}
	return context.WithValue(ctx, sandboxKey{}, root)
}

// WithExtraSandboxRoots 追加文档等非源码沙箱根目录（路径须位于其中任一目录才允许访问）。
func WithExtraSandboxRoots(ctx context.Context, roots []string) context.Context {
	var cleaned []string
	for _, root := range roots {
		if r := normalizeSandboxRoot(root); r != "" {
			cleaned = append(cleaned, r)
		}
	}
	if len(cleaned) == 0 {
		return ctx
	}
	return context.WithValue(ctx, extraSandboxKey{}, cleaned)
}

// SandboxFrom 返回当前 context 绑定的沙箱根目录（绝对路径）。
func SandboxFrom(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	root, _ := ctx.Value(sandboxKey{}).(string)
	return root
}

// ExtraSandboxRootsFrom 返回额外沙箱根目录列表。
func ExtraSandboxRootsFrom(ctx context.Context) []string {
	if ctx == nil {
		return nil
	}
	roots, _ := ctx.Value(extraSandboxKey{}).([]string)
	return roots
}

// normalizeSandboxRoot 规范化沙箱根目录路径。
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

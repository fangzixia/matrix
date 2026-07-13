// Package access 定义宿主注入的文件系统访问策略；tools 仅校验路径是否在允许范围内。
package access

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
)

type policyKey struct{}

// Policy 描述一次 Run 内文件类工具可访问的路径与可选运行时目录。
type Policy struct {
	// Roots 为允许读写的根目录列表（绝对路径，已规范化）。
	Roots []string
	// WorkDir 为 shell 类工具的工作目录；空则使用 Roots[0]。
	WorkDir string
	// ScratchDir 为可选运行时写入目录（todo、大输出 spill 等）；空则相关能力不可用。
	ScratchDir string
}

// WithPolicy 将访问策略绑定到 context。
func WithPolicy(ctx context.Context, p Policy) context.Context {
	p = normalizePolicy(p)
	return context.WithValue(ctx, policyKey{}, p)
}

// PolicyFrom 读取 context 中的访问策略。
func PolicyFrom(ctx context.Context) (Policy, bool) {
	p, ok := ctx.Value(policyKey{}).(Policy)
	return p, ok
}

// NewPolicy 由宿主组装策略：roots 为可访问目录，scratchDir 为可选运行时目录。
func NewPolicy(roots []string, scratchDir string) Policy {
	return normalizePolicy(Policy{Roots: roots, ScratchDir: scratchDir})
}

func normalizePolicy(p Policy) Policy {
	var roots []string
	for _, r := range p.Roots {
		if nr := NormalizeRoot(r); nr != "" {
			roots = append(roots, nr)
		}
	}
	p.Roots = roots
	if p.WorkDir != "" {
		p.WorkDir = NormalizeRoot(p.WorkDir)
	} else if len(p.Roots) > 0 {
		p.WorkDir = p.Roots[0]
	}
	p.ScratchDir = NormalizeRoot(p.ScratchDir)
	return p
}

// NormalizeRoot 规范化根目录路径。
func NormalizeRoot(root string) string {
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

// WorkDir 返回 shell 工具应使用的当前工作目录。
func WorkDir(ctx context.Context) string {
	if p, ok := PolicyFrom(ctx); ok && p.WorkDir != "" {
		return p.WorkDir
	}
	return ""
}

// ScratchDir 返回运行时写入目录；未配置时返回错误。
func ScratchDir(ctx context.Context) (string, error) {
	p, ok := PolicyFrom(ctx)
	if !ok || p.ScratchDir == "" {
		return "", fmt.Errorf("运行时写入目录未配置")
	}
	return p.ScratchDir, nil
}

// PathParamDesc 为文件/目录类工具的 path 参数说明。
const PathParamDesc = "必填。要访问的文件或目录的绝对路径（如 Windows: C:\\\\proj\\\\src）；不可使用相对路径。"

// ReadPathParamDesc 为 read_file 的 path 参数说明。
const ReadPathParamDesc = "必填。要读取文件的绝对路径；不可使用相对路径。"

// ErrPathRequired 表示未提供 path。
var ErrPathRequired = fmt.Errorf("必须明确指定 path 参数，不可省略或使用 \".\"")

// ErrPathNotAbsolute 表示 path 不是绝对路径。
var ErrPathNotAbsolute = fmt.Errorf("path 必须为绝对路径，不可使用相对路径")

// ErrAccessDenied 表示路径不在允许范围内。
var ErrAccessDenied = fmt.Errorf("路径不在允许访问的目录内")

// RequirePath 校验 path 已由调用方显式提供。
func RequirePath(path string) error {
	p := strings.TrimSpace(path)
	if p == "" || p == "." {
		return ErrPathRequired
	}
	return nil
}

// ResolveAbsolute 仅接受绝对路径，返回规范化后的绝对路径。
func ResolveAbsolute(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("路径为空")
	}
	if !filepath.IsAbs(path) {
		return "", ErrPathNotAbsolute
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("无效路径: %w", err)
	}
	return filepath.Clean(abs), nil
}

// ResolveAllowed 校验 path 为绝对路径且位于 Policy.Roots 内。
func ResolveAllowed(ctx context.Context, path string) (string, error) {
	if err := RequirePath(path); err != nil {
		return "", err
	}
	resolved, err := ResolveAbsolute(path)
	if err != nil {
		return "", err
	}
	if err := CheckAllowed(ctx, resolved); err != nil {
		return "", err
	}
	return resolved, nil
}

// CheckAllowed 校验绝对路径位于任一允许根目录下。
func CheckAllowed(ctx context.Context, absPath string) error {
	p, ok := PolicyFrom(ctx)
	if !ok || len(p.Roots) == 0 {
		return fmt.Errorf("文件访问策略未配置，拒绝访问")
	}
	abs, err := filepath.Abs(absPath)
	if err != nil {
		return fmt.Errorf("无效路径: %w", err)
	}
	for _, root := range p.Roots {
		if WithinRoot(abs, root) {
			return nil
		}
	}
	return fmt.Errorf("%w: %s", ErrAccessDenied, abs)
}

// WithinRoot 判断 path 是否位于 root 下（含 root 自身）。
func WithinRoot(path, root string) bool {
	path = filepath.Clean(path)
	root = filepath.Clean(root)
	if path == root {
		return true
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// ToAbsolutePath 将路径转为绝对路径；相对路径则相对于 base 拼接。
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

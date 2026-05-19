package tools

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// NewGlobTool 创建按 Glob 模式匹配文件路径的工具。
//
// 职责边界：仅按文件名模式枚举路径，不涉及文件内容。
// 内容级检索请使用 [NewGrepTool]。
//
// 对标 claude-code GlobTool：
//   - 支持标准 Glob（*, ?, [...]）和递归通配符 **
//   - 结果限制 100 条（超出时提示截断）
//   - 返回相对于工作目录的路径（节省 token）
//   - 并发安全（只读操作）
func NewGlobTool() *Tool {
	return &Tool{
		Name:        "glob",
		Description: "按 Glob 模式搜索文件名（路径级匹配，不搜索文件内容）。支持 * ? [...] 和 **。最多返回 100 条。与 grep 不同：grep 在文件内容里找正则。",
		Parameters: JSONSchema{
			Type: "object",
			Properties: map[string]PropSchema{
				"pattern": {
					Type:        "string",
					Description: "Glob 模式，如 \"**/*.go\"、\"src/**/*.ts\"、\"*.json\"",
				},
				"path": {
					Type:        "string",
					Description: "搜索的根目录（默认为当前工作目录）",
				},
			},
			Required: []string{"pattern"},
		},
		IsConcurrencySafe: true,
		Execute:           execGlob,
	}
}

const globMaxResults = 100

func execGlob(_ context.Context, args map[string]any) (string, error) {
	pattern, _ := getString(args, "pattern")
	root, _ := getString(args, "path")

	if root == "" {
		var err error
		root, err = os.Getwd()
		if err != nil {
			return "", fmt.Errorf("glob: 获取工作目录失败: %w", err)
		}
	} else if !filepath.IsAbs(root) {
		cwd, _ := os.Getwd()
		root = filepath.Join(cwd, root)
	}

	matches, truncated, err := globSearch(pattern, root, globMaxResults)
	if err != nil {
		return "", fmt.Errorf("glob: %w", err)
	}

	cwd, _ := os.Getwd()
	rel := make([]string, 0, len(matches))
	for _, m := range matches {
		if r, err := filepath.Rel(cwd, m); err == nil {
			rel = append(rel, r)
		} else {
			rel = append(rel, m)
		}
	}

	if len(rel) == 0 {
		return "未找到匹配文件", nil
	}

	var sb strings.Builder
	for _, f := range rel {
		sb.WriteString(f)
		sb.WriteByte('\n')
	}
	if truncated {
		sb.WriteString("（结果已截断，最多显示 100 条。请使用更精确的路径或模式）")
	}
	return sb.String(), nil
}

// globSearch 在 root 下按 pattern 搜索文件，返回绝对路径列表。
func globSearch(pattern, root string, limit int) (matches []string, truncated bool, err error) {
	if !strings.Contains(pattern, "**") {
		fullPattern := pattern
		if !filepath.IsAbs(pattern) {
			fullPattern = filepath.Join(root, pattern)
		}
		files, err := filepath.Glob(fullPattern)
		if err != nil {
			return nil, false, err
		}
		if len(files) > limit {
			return files[:limit], true, nil
		}
		return files, false, nil
	}

	suffixPattern := strings.TrimPrefix(pattern, "**/")
	if suffixPattern == pattern {
		idx := strings.Index(pattern, "**")
		suffixPattern = pattern[idx+3:]
	}

	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, werr error) error {
		if werr != nil {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if name != "." && (strings.HasPrefix(name, ".") ||
				name == "node_modules" || name == "vendor") {
				return filepath.SkipDir
			}
			return nil
		}
		matched, err := filepath.Match(suffixPattern, filepath.Base(path))
		if err != nil {
			return err
		}
		if matched {
			matches = append(matches, path)
			if len(matches) >= limit+1 {
				return errStopGlobWalk
			}
		}
		return nil
	})

	if err == errStopGlobWalk {
		return matches[:limit], true, nil
	}
	return matches, false, err
}

// errStopGlobWalk 终止 glob 的 WalkDir。
var errStopGlobWalk = fmt.Errorf("glob_stop_walk")

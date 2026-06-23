package tools

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
)

// NewGlobTool 创建按 Glob 模式匹配文件路径的工具。
//
// 职责边界：仅按文件名模式枚举路径，不涉及文件内容。
// 内容级检索请使用 [NewGrepTool]。
//
// GlobTool：
//   - 支持标准 Glob（*, ?, [...]）和递归通配符 **
//   - 结果限制 100 条（超出时提示截断）
//   - 返回匹配文件的绝对路径
//   - 并发安全（只读操作）
func NewGlobTool() *Tool {
	return &Tool{
		Name:        "glob",
		Description: "在指定目录下按 Glob 模式搜索文件名；必须在 path 中写明搜索根目录。最多返回 100 条。",
		Parameters: JSONSchema{
			Type: "object",
			Properties: map[string]PropSchema{
				"pattern": {
					Type:        "string",
					Description: "Glob 模式，如 \"**/*.go\"、\"src/**/*.ts\"、\"*.json\"",
				},
				"path": {
					Type:        "string",
					Description: pathParamDesc,
				},
			},
			Required: []string{"pattern", "path"},
		},
		IsConcurrencySafe: true,
		Execute:           execGlob,
	}
}

const globMaxResults = 100

// execGlob 在沙箱内执行 glob 文件匹配。
func execGlob(ctx context.Context, args map[string]any) (string, error) {
	pattern, _ := getString(args, "pattern")
	searchPath, _ := getString(args, "path")
	searchRoot, err := ResolveAndValidateToolPath(ctx, searchPath)
	if err != nil {
		return "", fmt.Errorf("glob: %w", err)
	}
	matches, truncated, err := globSearch(pattern, searchRoot, globMaxResults)
	if err != nil {
		return "", fmt.Errorf("glob: %w", err)
	}
	if len(matches) == 0 {
		return "未找到匹配文件", nil
	}
	var sb strings.Builder
	for _, f := range matches {
		f = ToAbsolutePath(f, searchRoot)
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

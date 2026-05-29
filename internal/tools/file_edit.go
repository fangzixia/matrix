package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// NewFileEditTool 创建文件精确字符串替换工具。
//
// FileEditTool（工具名 str_replace_editor）。
// 核心设计：old_string → new_string 原地替换，无需重写整个文件。
//
// 行为规则（与源码对齐）：
//   - old_string 为空且文件不存在 → 创建新文件，内容为 new_string
//   - old_string 为空且文件存在且为空 → 写入 new_string
//   - old_string 不在文件中 → 返回错误
//   - old_string 有多处匹配且 replace_all=false → 返回错误（要求更多上下文）
//   - replace_all=true → 替换所有匹配
func NewFileEditTool() *Tool {
	return &Tool{
		Name: "str_replace_editor",
		Description: `对文件执行精确的字符串替换。规则：
1. old_string 必须与文件内容完全匹配（含空格和换行）。
2. old_string 为空字符串时，若文件不存在则创建；若文件为空则写入 new_string。
3. 若 old_string 有多处匹配而 replace_all 为 false，则报错（需增加上下文使 old_string 唯一）。
4. 建议调用前先用 read_file 读取文件，确认 old_string 内容准确。`,
		Parameters: JSONSchema{
			Type: "object",
			Properties: map[string]PropSchema{
				"file_path": {
					Type:        "string",
					Description: pathParamDesc,
				},
				"old_string": {
					Type:        "string",
					Description: "要被替换的精确字符串（含完整缩进和换行）。空字符串表示创建新文件或向空文件写入内容",
				},
				"new_string": {
					Type:        "string",
					Description: "替换后的新内容",
				},
				"replace_all": {
					Type:        "boolean",
					Description: "true 时替换全部匹配；false（默认）时若有多处匹配则报错",
				},
			},
			Required: []string{"file_path", "old_string", "new_string"},
		},
		IsConcurrencySafe: false,
		Execute:           execFileEdit,
	}
}

// execFileEdit 是 str_replace_editor 工具的执行逻辑。
func execFileEdit(_ context.Context, args map[string]any) (string, error) {
	filePath, _ := getString(args, "file_path")
	oldStr, _ := getString(args, "old_string")
	newStr, _ := getString(args, "new_string")
	replaceAll, _ := args["replace_all"].(bool)

	resolvedPath, resolveErr := ResolveAndValidateToolPath(filePath)
	if resolveErr != nil {
		return "", fmt.Errorf("str_replace_editor: %w", resolveErr)
	}
	filePath = resolvedPath

	// 读取现有内容。所有写入路径均由统一工作区策略约束。
	content, fileExists, err := readTextFile(filePath)
	if err != nil {
		return "", fmt.Errorf("str_replace_editor: 读取文件失败: %w", err)
	}

	// ── old_string 为空：创建文件或写入空文件 ──────────────────────────
	if oldStr == "" {
		if fileExists && strings.TrimSpace(content) != "" {
			return "", fmt.Errorf(
				"str_replace_editor: 文件 %s 已存在且非空。若要替换内容，请提供非空 old_string",
				filePath)
		}
		if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
			return "", fmt.Errorf("str_replace_editor: 创建目录失败: %w", err)
		}
		if err := os.WriteFile(filePath, []byte(newStr), 0o644); err != nil {
			return "", fmt.Errorf("str_replace_editor: 写入失败: %w", err)
		}
		if fileExists {
			return fmt.Sprintf("已更新 %s（原文件为空）", filePath), nil
		}
		return fmt.Sprintf("已创建 %s", filePath), nil
	}

	// ── 文件不存在 ─────────────────────────────────────────────────────
	if !fileExists {
		return "", fmt.Errorf("str_replace_editor: 文件不存在: %s", filePath)
	}

	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		return "", fmt.Errorf("str_replace_editor: 创建目录失败: %w", err)
	}

	// ── 检查 old_string 是否存在 ────────────────────────────────────────
	if !strings.Contains(content, oldStr) {
		// 仿照源码给出详细错误：显示 old_string 内容帮助调试。
		preview := oldStr
		return "", fmt.Errorf(
			"str_replace_editor: 未在文件 %s 中找到要替换的字符串:\n%s",
			filePath, preview)
	}

	// ── 检查匹配数量 ─────────────────────────────────────────────────────
	matches := strings.Count(content, oldStr)
	if matches > 1 && !replaceAll {
		return "", fmt.Errorf(
			"str_replace_editor: 找到 %d 处匹配，但 replace_all=false。\n"+
				"请在 old_string 中增加更多上下文（前后几行）使其唯一，或设置 replace_all=true",
			matches)
	}

	// ── 执行替换 ─────────────────────────────────────────────────────────
	var newContent string
	if replaceAll {
		newContent = strings.ReplaceAll(content, oldStr, newStr)
	} else {
		newContent = strings.Replace(content, oldStr, newStr, 1)
	}

	if err := os.WriteFile(filePath, []byte(newContent), 0o644); err != nil {
		return "", fmt.Errorf("str_replace_editor: 写入失败: %w", err)
	}

	if replaceAll && matches > 1 {
		return fmt.Sprintf("已更新 %s（替换了 %d 处匹配）", filePath, matches), nil
	}
	return fmt.Sprintf("已更新 %s", filePath), nil
}

// readTextFile 读取文本文件内容；文件不存在时返回 ("", false, nil)。
func readTextFile(path string) (content string, exists bool, err error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return string(data), true, nil
}

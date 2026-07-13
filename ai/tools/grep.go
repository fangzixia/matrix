package tools

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"matrix/ai/access"
	"matrix/ai/util"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// NewGrepTool 创建在文件内容中搜索正则的工具。
//
// 职责边界：在文件内容中匹配正则表达式。
// 按路径/文件名枚举请使用 [NewGlobTool]，二者互补而非同一工具。
//
// GrepTool：
//   - 优先调用系统 ripgrep（rg）
//   - rg 不可用时降级为纯 Go
//   - output_mode: files_with_matches / content / count
//   - 支持 -i、-C、-n、head_limit
//   - 并发安全（只读）
func NewGrepTool() *Tool {
	return &Tool{
		Name:        "grep",
		Description: "在文件内容中搜索正则表达式。优先使用 ripgrep（rg），不可用时纯 Go 实现。与 glob 不同：glob 只按文件名模式列路径，不读文件内容。",
		Parameters: JSONSchema{
			Type: "object",
			Properties: map[string]PropSchema{
				"pattern": {
					Type:        "string",
					Description: "正则表达式（ripgrep 语法）",
				},
				"path": {
					Type:        "string",
					Description: access.PathParamDesc,
				},
				"glob": {
					Type:        "string",
					Description: "文件过滤 Glob，如 \"*.go\"（映射到 rg --glob）",
				},
				"output_mode": {
					Type:        "string",
					Description: "\"files_with_matches\"（默认）、\"content\"、\"count\"",
				},
				"-i": {
					Type:        "boolean",
					Description: "大小写不敏感",
				},
				"-C": {
					Type:        "integer",
					Description: "匹配行前后各 N 行上下文（content 模式）",
				},
				"-n": {
					Type:        "boolean",
					Description: "显示行号（content 模式，默认 true）",
				},
				"head_limit": {
					Type:        "integer",
					Description: "最多返回条数（默认 250，0 表示不限）",
				},
			},
			Required: []string{"pattern", "path"},
		},
		IsConcurrencySafe: true,
		StatusLabel: func(args map[string]any) string {
			pattern, _ := getString(args, "pattern")
			path, _ := getString(args, "path")
			return fmt.Sprintf("grep %q @ %s …", pattern, path)
		},
		Execute: execGrep,
	}
}

const grepDefaultHeadLimit = 250

var grepHasRipgrep = func() bool {
	_, err := exec.LookPath("rg")
	return err == nil
}()

// execGrep 在沙箱内执行内容搜索（优先 rg）。
func execGrep(ctx context.Context, args map[string]any) (string, error) {
	pattern, _ := getString(args, "pattern")
	path, _ := getString(args, "path")
	globFilter, _ := getString(args, "glob")
	outputMode, _ := getString(args, "output_mode")
	caseInsensitive, _ := args["-i"].(bool)
	contextLines, _ := args["-C"].(float64)
	showLineNumbers := true
	if v, ok := args["-n"].(bool); ok {
		showLineNumbers = v
	}
	headLimit := grepDefaultHeadLimit
	if v, ok := args["head_limit"].(float64); ok {
		if int(v) == 0 {
			headLimit = 0
		} else if int(v) > 0 {
			headLimit = int(v)
		}
	}
	if outputMode == "" {
		outputMode = "files_with_matches"
	}
	var err error
	searchRoot, err := access.ResolveAllowed(ctx, path)
	if err != nil {
		return "", fmt.Errorf("grep: %w", err)
	}
	var out string
	if grepHasRipgrep {
		out, err = execGrepRg(ctx, pattern, searchRoot, globFilter, outputMode,
			caseInsensitive, int(contextLines), showLineNumbers, headLimit)
	} else {
		out, err = execGrepGo(ctx, pattern, searchRoot, globFilter, outputMode,
			caseInsensitive, int(contextLines), showLineNumbers, headLimit)
	}
	if err == nil && out != "" {
		util.WriteChunks(ctx, out, defaultEmitChunkSize)
	}
	return out, err
}

// execGrepRg 使用 ripgrep 执行搜索。
func execGrepRg(
	ctx context.Context,
	pattern, path, globFilter, outputMode string,
	caseInsensitive bool, contextLines int, showLineNumbers bool, headLimit int,
) (string, error) {
	rgArgs := []string{"--hidden", "--max-columns", "500"}
	for _, dir := range []string{".git", ".svn", ".hg"} {
		rgArgs = append(rgArgs, "--glob", "!"+dir)
	}
	if caseInsensitive {
		rgArgs = append(rgArgs, "-i")
	}
	switch outputMode {
	case "files_with_matches":
		rgArgs = append(rgArgs, "-l")
	case "count":
		rgArgs = append(rgArgs, "-c")
	case "content":
		if showLineNumbers {
			rgArgs = append(rgArgs, "-n")
		}
		if contextLines > 0 {
			rgArgs = append(rgArgs, "-C", fmt.Sprintf("%d", contextLines))
		}
	}
	if globFilter != "" {
		rgArgs = append(rgArgs, "--glob", globFilter)
	}
	if strings.HasPrefix(pattern, "-") {
		rgArgs = append(rgArgs, "-e", pattern)
	} else {
		rgArgs = append(rgArgs, pattern)
	}
	rgArgs = append(rgArgs, path)
	cmd := exec.CommandContext(ctx, "rg", rgArgs...)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok && exitErr.ExitCode() == 1 {
			return formatGrepResult(outputMode, []string{}, 0), nil
		}
		return "", fmt.Errorf("grep(rg): %w", err)
	}
	lines := grepSplitLines(out.String())
	lines = grepApplyHeadLimit(lines, headLimit)
	lines = grepAbsolutizePaths(lines, path, outputMode)
	return formatGrepResult(outputMode, lines, len(lines)), nil
}

// execGrepGo 使用 Go 原生实现执行搜索。
func execGrepGo(
	ctx context.Context,
	pattern, rootPath, globFilter, outputMode string,
	caseInsensitive bool, contextLines int, showLineNumbers bool, headLimit int,
) (string, error) {
	reStr := pattern
	if caseInsensitive {
		reStr = "(?i)" + reStr
	}
	re, err := regexp.Compile(reStr)
	if err != nil {
		return "", fmt.Errorf("grep: 正则编译失败: %w", err)
	}
	var (
		matchFiles   []string
		contentLines []string
		countLines   []string
		totalFiles   int
	)
	err = filepath.WalkDir(rootPath, func(path string, d fs.DirEntry, werr error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
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
		if globFilter != "" {
			matched, _ := filepath.Match(globFilter, d.Name())
			if !matched {
				return nil
			}
		}
		lines, err := grepSearchFile(path, re, outputMode, contextLines, showLineNumbers)
		if err != nil {
			return nil
		}
		if len(lines) == 0 {
			return nil
		}
		absPath := access.ToAbsolutePath(path, rootPath)
		switch outputMode {
		case "files_with_matches":
			matchFiles = append(matchFiles, absPath)
		case "content":
			contentLines = append(contentLines, lines...)
		case "count":
			countLines = append(countLines, fmt.Sprintf("%s:%d", absPath, len(lines)))
		}
		totalFiles++
		return nil
	})
	if err != nil && err != context.Canceled {
		return "", fmt.Errorf("grep: %w", err)
	}
	switch outputMode {
	case "files_with_matches":
		lines := grepApplyHeadLimit(matchFiles, headLimit)
		return formatGrepResult(outputMode, lines, len(lines)), nil
	case "content":
		lines := grepApplyHeadLimit(contentLines, headLimit)
		return formatGrepResult(outputMode, lines, totalFiles), nil
	case "count":
		lines := grepApplyHeadLimit(countLines, headLimit)
		return formatGrepResult(outputMode, lines, totalFiles), nil
	default:
		return formatGrepResult("files_with_matches", matchFiles, len(matchFiles)), nil
	}
}

// grepSearchFile 在单个文件中搜索匹配行。
func grepSearchFile(
	path string, re *regexp.Regexp, outputMode string,
	contextLines int, showLineNumbers bool,
) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var fileLines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fileLines = append(fileLines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	var results []string
	for i, line := range fileLines {
		if !re.MatchString(line) {
			continue
		}
		if outputMode == "files_with_matches" {
			return []string{"match"}, nil
		}
		if outputMode == "count" {
			results = append(results, line)
			continue
		}
		start := maxInt(0, i-contextLines)
		end := minInt(len(fileLines)-1, i+contextLines)
		for j := start; j <= end; j++ {
			if showLineNumbers {
				results = append(results, fmt.Sprintf("%d:%s", j+1, fileLines[j]))
			} else {
				results = append(results, fileLines[j])
			}
		}
	}
	return results, nil
}

// grepSplitLines 拆分 grep 结果行。
func grepSplitLines(s string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.TrimRight(s, "\n\r")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

// grepApplyHeadLimit 对 grep 结果应用 head 限制。
func grepApplyHeadLimit(lines []string, limit int) []string {
	if limit == 0 || len(lines) <= limit {
		return lines
	}
	return lines[:limit]
}

// grepSplitPathAndRest 从 ripgrep 输出行中分离文件路径与后续部分（行号/内容）。
// Windows 绝对路径含盘符冒号（如 E:\foo.go:30:line），不可直接用首个 ':' 分割。
func grepSplitPathAndRest(line string) (filePath, rest string, ok bool) {
	if line == "" {
		return "", "", false
	}
	splitAt := grepPathLineSeparator(line)
	if splitAt < 0 {
		return line, "", false
	}
	return line[:splitAt], line[splitAt:], true
}

// grepPathLineSeparator 返回 ripgrep 行中「文件路径」与「行号:内容」之间的 ':' 位置。
func grepPathLineSeparator(line string) int {
	if len(line) >= 2 && line[1] == ':' {
		c := line[0]
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') {
			if idx := strings.Index(line[2:], ":"); idx >= 0 {
				return 2 + idx
			}
			return -1
		}
	}
	if idx := strings.Index(line, ":"); idx > 0 {
		return idx
	}
	return -1
}

// grepAbsolutizePathPart 将 grep 结果中的路径部分转为绝对路径；已是绝对路径则仅规范化。
func grepAbsolutizePathPart(path, searchRoot string) string {
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
	return access.ToAbsolutePath(path, searchRoot)
}

// grepAbsolutizePaths 将 grep 结果路径转为绝对路径。
func grepAbsolutizePaths(lines []string, searchRoot, outputMode string) []string {
	out := make([]string, len(lines))
	for i, line := range lines {
		switch outputMode {
		case "files_with_matches":
			out[i] = grepAbsolutizePathPart(line, searchRoot)
		default:
			filePath, rest, ok := grepSplitPathAndRest(line)
			if ok {
				out[i] = grepAbsolutizePathPart(filePath, searchRoot) + rest
			} else {
				out[i] = line
			}
		}
	}
	return out
}

// formatGrepResult 格式化 grep 搜索结果文本。
func formatGrepResult(outputMode string, lines []string, count int) string {
	if len(lines) == 0 {
		return "未找到匹配"
	}
	switch outputMode {
	case "files_with_matches":
		return fmt.Sprintf("找到 %d 个文件\n%s", count, strings.Join(lines, "\n"))
	case "count":
		return fmt.Sprintf("匹配统计（%d 个文件）:\n%s", count, strings.Join(lines, "\n"))
	default:
		return strings.Join(lines, "\n")
	}
}

// maxInt / minInt 供本包多处使用（grep、web_search 等），避免与标准库 min/max 冲突。
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// minInt 返回两个整数中的较小值。
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

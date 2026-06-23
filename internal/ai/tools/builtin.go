package tools

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// ReadFile 读取沙箱内指定文件的文本内容。
var ReadFile = &Tool{
	Name:        "read_file",
	Description: "读取沙箱内指定路径的文件内容（文本）。",
	Parameters: JSONSchema{
		Type: "object",
		Properties: map[string]PropSchema{
			"path": {Type: "string", Description: readPathParamDesc},
		},
		Required: []string{"path"},
	},
	IsConcurrencySafe: true,
	Execute: func(ctx context.Context, args map[string]any) (string, error) {
		path, ok := getString(args, "path")
		if !ok {
			return "", fmt.Errorf("read_file: 缺少参数 'path'")
		}
		targetPath, resolveErr := ResolveAndValidateToolPath(ctx, path)
		if resolveErr != nil {
			return "", fmt.Errorf("read_file: %w", resolveErr)
		}
		data, err := os.ReadFile(targetPath)
		if err != nil {
			return "", fmt.Errorf("read_file: %w", err)
		}
		return string(data), nil
	},
}

// WriteFile 将文本内容写入沙箱内指定文件（覆盖写入）。
var WriteFile = &Tool{
	Name:        "write_file",
	Description: "将文本内容写入沙箱内指定路径的文件（覆盖写入）。",
	Parameters: JSONSchema{
		Type: "object",
		Properties: map[string]PropSchema{
			"path":    {Type: "string", Description: pathParamDesc},
			"content": {Type: "string", Description: "要写入的文本内容"},
		},
		Required: []string{"path", "content"},
	},
	IsConcurrencySafe: false,
	Execute: func(ctx context.Context, args map[string]any) (string, error) {
		path, _ := getString(args, "path")
		content, _ := getString(args, "content")
		targetPath, resolveErr := ResolveAndValidateToolPath(ctx, path)
		if resolveErr != nil {
			return "", fmt.Errorf("write_file: %w", resolveErr)
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return "", fmt.Errorf("write_file: 创建目录失败: %w", err)
		}
		if err := os.WriteFile(targetPath, []byte(content), 0o644); err != nil {
			return "", fmt.Errorf("write_file: %w", err)
		}
		return fmt.Sprintf("已写入 %d 字节到 %s", len(content), targetPath), nil
	},
}

// ListDir 列出沙箱内指定目录下的文件与子目录。
var ListDir = &Tool{
	Name:        "list_dir",
	Description: "列出沙箱内指定目录下的条目；path 须为绝对路径。",
	Parameters: JSONSchema{
		Type: "object",
		Properties: map[string]PropSchema{
			"path": {Type: "string", Description: pathParamDesc},
		},
		Required: []string{"path"},
	},
	IsConcurrencySafe: true,
	Execute: func(ctx context.Context, args map[string]any) (string, error) {
		path, _ := getString(args, "path")
		targetPath, resolveErr := ResolveAndValidateToolPath(ctx, path)
		if resolveErr != nil {
			return "", fmt.Errorf("list_dir: %w", resolveErr)
		}
		entries, err := os.ReadDir(targetPath)
		if err != nil {
			return "", fmt.Errorf("list_dir: %w", err)
		}
		var sb strings.Builder
		for _, e := range entries {
			info, _ := e.Info()
			size := int64(0)
			if info != nil {
				size = info.Size()
			}
			entryPath := filepath.Join(targetPath, e.Name())
			if e.IsDir() {
				sb.WriteString(fmt.Sprintf("[目录] %s/\n", entryPath))
			} else {
				sb.WriteString(fmt.Sprintf("[文件] %s  (%d B)\n", entryPath, size))
			}
		}
		return sb.String(), nil
	},
}

// Bash 在非 Windows 上通过 `sh -c` 执行；在 Windows 上通过 **CMD**（cmd.exe /C）执行。
// Windows 环境通常没有 bash，请使用 cmd 语法而非 bash 语法。
var Bash = &Tool{
	Name: "bash",
	Description: `执行 shell 命令。非 Windows 使用 /bin/sh -c；Windows 使用 **cmd.exe /C**（CMD 语法，非 Linux bash）。
Windows 上请用 where、more、type、dir 等命令，勿用 which、tail、cat 等 Unix 命令。
若需 PowerShell（如 Select-Object、$ 变量），请改用 **powershell** 工具。`,
	Parameters: JSONSchema{
		Type: "object",
		Properties: map[string]PropSchema{
			"command": {Type: "string", Description: "要执行的命令（Windows 下为 CMD 语法）"},
		},
		Required: []string{"command"},
	},
	IsConcurrencySafe: false,
	Execute: func(ctx context.Context, args map[string]any) (string, error) {
		command, ok := getString(args, "command")
		if !ok || command == "" {
			return "", fmt.Errorf("bash: 缺少参数 'command'")
		}
		var cmd *exec.Cmd
		if runtime.GOOS == "windows" {
			shell := os.Getenv("COMSPEC")
			if shell == "" {
				shell = "cmd.exe"
			}
			cmd = exec.CommandContext(ctx, shell, "/C", command)
		} else {
			cmd = exec.CommandContext(ctx, "sh", "-c", command)
		}
		if dir := SandboxFrom(ctx); dir != "" {
			cmd.Dir = dir
		}
		var out bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &out
		err := cmd.Run()
		result := out.String()
		if err != nil {
			return result, fmt.Errorf("命令失败: %w", err)
		}
		return result, nil
	},
}

// PowerShell 在 Windows 上通过 powershell -NoProfile -Command 执行。
// 适用于需要 PowerShell  cmdlet 的场景，与 bash（CMD）工具互补。
var PowerShell = &Tool{
	Name:        "powershell",
	Description: "在 Windows 上执行 PowerShell 命令（-NoProfile -Command）；仅 Windows 可用。",
	Parameters: JSONSchema{
		Type: "object",
		Properties: map[string]PropSchema{
			"command": {Type: "string", Description: "PowerShell 命令或脚本"},
		},
		Required: []string{"command"},
	},
	IsConcurrencySafe: false,
	Execute: func(ctx context.Context, args map[string]any) (string, error) {
		if runtime.GOOS != "windows" {
			return "", fmt.Errorf("powershell: 仅支持 Windows")
		}
		command, ok := getString(args, "command")
		if !ok || command == "" {
			return "", fmt.Errorf("powershell: 缺少参数 'command'")
		}
		cmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-Command", command)
		if dir := SandboxFrom(ctx); dir != "" {
			cmd.Dir = dir
		}
		var out bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &out
		err := cmd.Run()
		result := out.String()
		if err != nil {
			return result, fmt.Errorf("命令失败: %w", err)
		}
		return result, nil
	},
}

// DefaultRegistry 返回包含全部内置工具的 [Registry]。
//
// 注册工具：
//   - read_file          → ReadFile
//   - write_file         → WriteFile
//   - list_dir           → ListDir
//   - bash               → Bash（Windows 下为 CMD）
//   - powershell         → PowerShell（仅 Windows 注册）
//   - str_replace_editor → FileEditTool
//   - glob               → GlobTool（glob.go，按文件名模式搜索）
//   - grep               → GrepTool（grep.go，按内容搜索）
//   - web_fetch          → WebFetchTool（web_fetch.go）
//   - web_search         → WebSearchTool（web_search.go，需 BRAVE_SEARCH_API_KEY）
//   - sleep              → SleepTool（sleep.go）
//   - todo_write         → TodoWriteTool（todo_write.go）
func DefaultRegistry() *Registry {
	r := NewRegistry()
	r.Register(ReadFile)
	r.Register(WriteFile)
	r.Register(ListDir)
	r.Register(Bash)
	if runtime.GOOS == "windows" {
		r.Register(PowerShell)
	}
	r.Register(NewFileEditTool())
	r.Register(NewGlobTool())
	r.Register(NewGrepTool())
	r.Register(NewWebFetchTool())
	r.Register(NewWebSearchTool())
	r.Register(NewSleepTool())
	r.Register(NewTodoWriteTool())
	return r
}

// RegistryWithoutShell 返回不含 bash/powershell 的内置工具注册表。
func RegistryWithoutShell(_ *Registry) *Registry {
	r := NewRegistry()
	r.Register(ReadFile)
	r.Register(WriteFile)
	r.Register(ListDir)
	r.Register(NewFileEditTool())
	r.Register(NewGlobTool())
	r.Register(NewGrepTool())
	r.Register(NewWebFetchTool())
	r.Register(NewWebSearchTool())
	r.Register(NewSleepTool())
	r.Register(NewTodoWriteTool())
	return r
}

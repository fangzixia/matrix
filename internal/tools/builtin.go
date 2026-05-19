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

// ReadFile 是读取文件内容的内置工具（只读，并发安全）。
var ReadFile = &Tool{
	Name:        "read_file",
	Description: "读取指定路径文件的完整内容。",
	Parameters: JSONSchema{
		Type: "object",
		Properties: map[string]PropSchema{
			"path": {Type: "string", Description: "文件的绝对路径或相对路径。"},
		},
		Required: []string{"path"},
	},
	IsConcurrencySafe: true,
	Execute: func(ctx context.Context, args map[string]any) (string, error) {
		path, ok := getString(args, "path")
		if !ok || path == "" {
			return "", fmt.Errorf("read_file: 缺少必需参数 'path'")
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("read_file: %w", err)
		}
		return string(data), nil
	},
}

// WriteFile 是写入文件内容的内置工具，按需创建父目录（写操作，串行执行）。
var WriteFile = &Tool{
	Name:        "write_file",
	Description: "将内容写入文件，若文件或父目录不存在则自动创建。",
	Parameters: JSONSchema{
		Type: "object",
		Properties: map[string]PropSchema{
			"path":    {Type: "string", Description: "目标文件路径。"},
			"content": {Type: "string", Description: "要写入的文本内容。"},
		},
		Required: []string{"path", "content"},
	},
	IsConcurrencySafe: false,
	Execute: func(ctx context.Context, args map[string]any) (string, error) {
		path, _ := getString(args, "path")
		content, _ := getString(args, "content")
		if path == "" {
			return "", fmt.Errorf("write_file: 缺少必需参数 'path'")
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return "", fmt.Errorf("write_file: 创建父目录失败: %w", err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return "", fmt.Errorf("write_file: %w", err)
		}
		return fmt.Sprintf("已写入 %d 字节到 %s", len(content), path), nil
	},
}

// ListDir 是列出目录内容的内置工具（只读，并发安全）。
var ListDir = &Tool{
	Name:        "list_dir",
	Description: "列出指定目录下的文件和子目录。",
	Parameters: JSONSchema{
		Type: "object",
		Properties: map[string]PropSchema{
			"path": {Type: "string", Description: "目录路径，缺省时使用当前工作目录。"},
		},
	},
	IsConcurrencySafe: true,
	Execute: func(ctx context.Context, args map[string]any) (string, error) {
		path, _ := getString(args, "path")
		if path == "" {
			path = "."
		}
		entries, err := os.ReadDir(path)
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
			if e.IsDir() {
				sb.WriteString(fmt.Sprintf("[目录] %s/\n", e.Name()))
			} else {
				sb.WriteString(fmt.Sprintf("[文件] %s  (%d B)\n", e.Name(), size))
			}
		}
		return sb.String(), nil
	},
}

// Bash 在非 Windows 上执行 `sh -c`；在 Windows 上执行 **CMD**（cmd.exe /C），
// 对标 claude-code 在 Windows 上对「类 bash」工具采用 cmd 而非误称 bash。
var Bash = &Tool{
	Name: "bash",
	Description: `执行 shell 命令，返回合并后的标准输出与标准错误。
在非 Windows 系统上使用 /bin/sh -c；在 Windows 上使用 **cmd.exe /C**（CMD），并非 Linux bash。
Windows 上请避免 which、tail、cat 等典型 Unix 写法；可改用 where、more、type、dir。
需要 PowerShell（管道、Select-Object、$ 变量等）时请改用 **powershell** 工具。`,
	Parameters: JSONSchema{
		Type: "object",
		Properties: map[string]PropSchema{
			"command": {Type: "string", Description: "要执行的命令行（Windows 下为 CMD 语法）。"},
		},
		Required: []string{"command"},
	},
	IsConcurrencySafe: false,
	Execute: func(ctx context.Context, args map[string]any) (string, error) {
		command, ok := getString(args, "command")
		if !ok || command == "" {
			return "", fmt.Errorf("bash: 缺少必需参数 'command'")
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
		var out bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &out
		err := cmd.Run()
		result := out.String()
		if err != nil {
			return result, fmt.Errorf("命令退出异常: %w", err)
		}
		return result, nil
	},
}

// PowerShell 仅在 Windows 上可用：执行 powershell -NoProfile -Command。
// 对应需保留 PowerShell 语法的场景；一般文件与包管理优先用 bash（CMD）。
var PowerShell = &Tool{
	Name:        "powershell",
	Description: "在 Windows 上执行 PowerShell 命令（-NoProfile -Command）。非 Windows 系统上不可用。",
	Parameters: JSONSchema{
		Type: "object",
		Properties: map[string]PropSchema{
			"command": {Type: "string", Description: "PowerShell 命令文本。"},
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
			return "", fmt.Errorf("powershell: 缺少必需参数 'command'")
		}
		cmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-Command", command)
		var out bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &out
		err := cmd.Run()
		result := out.String()
		if err != nil {
			return result, fmt.Errorf("命令退出异常: %w", err)
		}
		return result, nil
	},
}

// DefaultRegistry 返回预装了所有通用工具的 [Registry]。
//
// 包含工具（对标 claude-code 第一档通用工具）：
//   - read_file       对应 FileReadTool
//   - write_file      对应 FileWriteTool
//   - list_dir        对应 LSDir（简化版）
//   - bash            对应 BashTool（Windows 下为 CMD）
//   - powershell      Windows 专用 PowerShell（非 Windows 不注册）
//   - str_replace_editor 对应 FileEditTool
//   - glob            对应 GlobTool（glob.go，仅路径/文件名模式）
//   - grep            对应 GrepTool（grep.go，文件内容正则）
//   - web_fetch       对应 WebFetchTool（web_fetch.go）
//   - web_search      对应 WebSearchTool（web_search.go，需 BRAVE_SEARCH_API_KEY）
//   - sleep           对应 SleepTool（sleep.go）
//   - todo_write      对应 TodoWriteTool（todo_write.go）
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

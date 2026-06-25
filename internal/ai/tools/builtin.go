package tools

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const readFileChunkSize = 8 * 1024

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
		EmitStatus(ctx, fmt.Sprintf("读取 %s …", targetPath))
		f, err := os.Open(targetPath)
		if err != nil {
			return "", fmt.Errorf("read_file: %w", err)
		}
		defer f.Close()
		var sb strings.Builder
		buf := make([]byte, readFileChunkSize)
		for {
			n, readErr := f.Read(buf)
			if n > 0 {
				chunk := string(buf[:n])
				sb.WriteString(chunk)
				EmitOutput(ctx, chunk)
			}
			if readErr == io.EOF {
				break
			}
			if readErr != nil {
				return sb.String(), fmt.Errorf("read_file: %w", readErr)
			}
		}
		return sb.String(), nil
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
		EmitStatus(ctx, fmt.Sprintf("写入 %s …", targetPath))
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return "", fmt.Errorf("write_file: 创建目录失败: %w", err)
		}
		if err := os.WriteFile(targetPath, []byte(content), 0o644); err != nil {
			return "", fmt.Errorf("write_file: %w", err)
		}
		msg := fmt.Sprintf("已写入 %d 字节到 %s", len(content), targetPath)
		EmitOutput(ctx, msg+"\n")
		return msg, nil
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
		EmitStatus(ctx, fmt.Sprintf("列出 %s …", targetPath))
		entries, err := os.ReadDir(targetPath)
		if err != nil {
			return "", fmt.Errorf("list_dir: %w", err)
		}
		var sb strings.Builder
		for i, e := range entries {
			info, _ := e.Info()
			size := int64(0)
			if info != nil {
				size = info.Size()
			}
			entryPath := filepath.Join(targetPath, e.Name())
			var line string
			if e.IsDir() {
				line = fmt.Sprintf("[目录] %s/\n", entryPath)
			} else {
				line = fmt.Sprintf("[文件] %s  (%d B)\n", entryPath, size)
			}
			sb.WriteString(line)
			if i%20 == 0 || i == len(entries)-1 {
				EmitOutput(ctx, line)
			}
		}
		return sb.String(), nil
	},
}

// Bash 在非 Windows 上通过 `sh -c` 执行；在 Windows 上通过 **CMD**（cmd.exe /C）执行。
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
		EmitStatus(ctx, fmt.Sprintf("$ %s", command))
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
		return runShellStreamed(ctx, cmd)
	},
}

// PowerShell 在 Windows 上通过 powershell -NoProfile -Command 执行。
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
		EmitStatus(ctx, fmt.Sprintf("PS> %s", command))
		cmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-Command", command)
		if dir := SandboxFrom(ctx); dir != "" {
			cmd.Dir = dir
		}
		return runShellStreamed(ctx, cmd)
	},
}

// DefaultRegistry 返回包含全部内置工具的 [Registry]。
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

// Package matrixpaths 定义 Matrix 应用数据路径。
//
// # 路径约定
//
// 所有 durable 数据位于 os.UserConfigDir()/matrix/（见 AppDataDir）：
//
//	{AppDataDir}/
//	  config.json                 — 应用配置（API Key、MCP 等）
//	  logs/matrix.log             — 运维日志
//	  workspaces/{workspace_id}/  — 按工作区隔离的运行时数据
//	    meta.json
//	    sessions/                 — Agent 审计 JSONL
//	    transcripts/              — Agent transcript（多轮对话）
//	    chat-history.json         — 对话历史列表（标题、展示消息）
//	    todos.json                — Agent todo_write 持久化
//	    subagents/                — 子 Agent sidechain
//	    exports/                  — 诊断导出默认目录
//
// 函数参数 workspacePath 指用户项目目录的绝对路径，用作查找 workspace_id 的键；
// 返回值均在应用数据目录下，不会写入用户项目目录。
package matrixpaths

import (
	"os"
	"path/filepath"
)

const (
	AppName = "matrix"

	DirSessions        = "sessions"
	DirChatTranscripts = "transcripts"
	DirSubagents       = "subagents"
	DirExports         = "exports"

	FileChatHistory = "chat-history.json"
	FileTodos       = "todos.json"
	DirLogs         = "logs"
	LogFileName     = "matrix.log"
)

// WorkspaceStoreRoot 返回 workspacePath 在应用数据下的存储根：AppData/workspaces/{id}。
func WorkspaceStoreRoot(workspacePath string) string {
	return workspaceStoreRoot(workspacePath)
}

// WorkspaceStoreJoin 在 WorkspaceStoreRoot 下拼接子路径。
func WorkspaceStoreJoin(workspacePath string, elems ...string) string {
	return joinWorkspaceStore(workspacePath, elems...)
}

// SessionsDir 返回 audit 会话目录。
func SessionsDir(workspacePath string) string {
	return WorkspaceStoreJoin(workspacePath, DirSessions)
}

// ChatTranscriptsDir 返回 Agent transcript 目录。
func ChatTranscriptsDir(workspacePath string) string {
	return WorkspaceStoreJoin(workspacePath, DirChatTranscripts)
}

// SubagentsDir 返回子 Agent sidechain 目录。
func SubagentsDir(workspacePath string) string {
	return WorkspaceStoreJoin(workspacePath, DirSubagents)
}

// ExportsDir 返回诊断导出默认目录。
func ExportsDir(workspacePath string) string {
	return WorkspaceStoreJoin(workspacePath, DirExports)
}

// ChatHistoryFile 返回对话历史列表文件路径。
func ChatHistoryFile(workspacePath string) string {
	return WorkspaceStoreJoin(workspacePath, FileChatHistory)
}

// TodosFile 返回 Agent todo_write 持久化文件路径。
func TodosFile(workspacePath string) string {
	return WorkspaceStoreJoin(workspacePath, FileTodos)
}

// LogsDir 返回应用级运维日志目录。
func LogsDir() (string, error) {
	app, err := AppDataDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(app, DirLogs)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

// LogFile 返回运维日志文件路径。
func LogFile() (string, error) {
	dir, err := LogsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, LogFileName), nil
}

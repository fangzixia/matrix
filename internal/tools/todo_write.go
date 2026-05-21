package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// TodoItem 表示单条 TODO 项。
type TodoItem struct {
	// ID 是 TODO 的唯一标识，用于 merge 操作时的匹配键。
	ID string `json:"id"`
	// Content 是 TODO 的描述文本。
	Content string `json:"content"`
	// Status 是当前状态：pending / in_progress / completed / cancelled。
	Status string `json:"status"`
}

// NewTodoWriteTool 创建结构化 TODO 列表管理工具。
//
// 职责边界：任务跟踪与持久化，与 [NewSleepTool] 无关。
//
// TodoWriteTool：
//   - merge=false：整表替换；merge=true：按 id 合并
//   - 持久化到工作目录 .taor-todos.json
func NewTodoWriteTool() *Tool {
	return &Tool{
		Name: "todo_write",
		Description: `创建和管理结构化 TODO 列表，用于追踪复杂任务的进度。
支持两种操作模式：
- merge=false（默认）：用 todos 数组完全替换现有列表
- merge=true：基于 id 字段增量合并（更新已有项，新 id 则插入）

参数 todos 必须为**数组**（每项含 id、content、status）。兼容旧版：若 API 仍传入 JSON 字符串也会被解析。
状态值：pending、in_progress、completed、cancelled`,
		Parameters: JSONSchema{
			Type: "object",
			Properties: map[string]PropSchema{
				"todos": {
					Type:        "array",
					Description: "TODO 项数组；每项须含 id、content、status",
					Items: &PropSchema{
						Type: "object",
						Properties: map[string]PropSchema{
							"id":      {Type: "string", Description: "唯一标识，用于 merge 匹配"},
							"content": {Type: "string", Description: "任务描述"},
							"status": {
								Type:        "string",
								Description: "pending | in_progress | completed | cancelled",
							},
						},
						Required: []string{"id", "content", "status"},
					},
				},
				"merge": {
					Type:        "boolean",
					Description: "true 时按 id 合并；false（默认）时完全替换列表",
				},
			},
			Required: []string{"todos"},
		},
		IsConcurrencySafe: false,
		Execute:           execTodoWrite,
	}
}

func todoWriteFilePath() string {
	root := getWorkspaceRoot()
	if root == "" {
		cwd, _ := os.Getwd()
		root = cwd
	}
	return filepath.Join(root, ".matrix-todos.json")
}

func decodeTodos(raw any) ([]TodoItem, error) {
	if raw == nil {
		return nil, fmt.Errorf("缺少 todos")
	}
	switch v := raw.(type) {
	case string:
		s := strings.TrimSpace(v)
		if s == "" {
			return nil, fmt.Errorf("todos 为空")
		}
		var items []TodoItem
		if err := json.Unmarshal([]byte(s), &items); err != nil {
			return nil, err
		}
		return items, nil
	case []any:
		data, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}
		var items []TodoItem
		if err := json.Unmarshal(data, &items); err != nil {
			return nil, err
		}
		return items, nil
	default:
		data, err := json.Marshal(raw)
		if err != nil {
			return nil, fmt.Errorf("todos 类型 %T 无法编码", raw)
		}
		var items []TodoItem
		if err := json.Unmarshal(data, &items); err != nil {
			return nil, err
		}
		return items, nil
	}
}

func execTodoWrite(_ context.Context, args map[string]any) (string, error) {
	raw, ok := args["todos"]
	if !ok {
		return "", fmt.Errorf("todo_write: 缺少必需参数 todos")
	}
	newItems, err := decodeTodos(raw)
	if err != nil {
		return "", fmt.Errorf("todo_write: 解析 todos 失败: %w", err)
	}
	merge := getBoolArg(args, "merge")

	validStatuses := map[string]bool{
		"pending": true, "in_progress": true,
		"completed": true, "cancelled": true,
	}
	for _, item := range newItems {
		if item.ID == "" {
			return "", fmt.Errorf("todo_write: TODO 项缺少 id 字段")
		}
		if !validStatuses[item.Status] {
			return "", fmt.Errorf("todo_write: 无效的 status %q", item.Status)
		}
	}

	var finalItems []TodoItem
	if merge {
		existing := todoWriteLoad()
		idxMap := make(map[string]int)
		for i, item := range existing {
			idxMap[item.ID] = i
		}
		finalItems = existing
		for _, newItem := range newItems {
			if idx, found := idxMap[newItem.ID]; found {
				finalItems[idx] = newItem
			} else {
				finalItems = append(finalItems, newItem)
			}
		}
	} else {
		finalItems = newItems
	}

	todoPath := todoWriteFilePath()
	if _, err := os.Stat(todoPath); os.IsNotExist(err) {
		if err := requireNewFileInWorkspace(todoPath); err != nil {
			return "", fmt.Errorf("todo_write: %w", err)
		}
	}
	if err := todoWriteSave(finalItems); err != nil {
		return "", fmt.Errorf("todo_write: 保存失败: %w", err)
	}
	return todoWriteFormat(finalItems), nil
}

func todoWriteLoad() []TodoItem {
	data, err := os.ReadFile(todoWriteFilePath())
	if err != nil {
		return nil
	}
	var items []TodoItem
	if err := json.Unmarshal(data, &items); err != nil {
		return nil
	}
	return items
}

func todoWriteSave(items []TodoItem) error {
	data, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(todoWriteFilePath(), data, 0o644)
}

func todoWriteFormat(items []TodoItem) string {
	if len(items) == 0 {
		return "TODO 列表已清空"
	}
	statusIcon := map[string]string{
		"pending": "○", "in_progress": "◐",
		"completed": "●", "cancelled": "✕",
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("TODO 列表（共 %d 项）:\n", len(items)))
	for _, item := range items {
		icon := statusIcon[item.Status]
		if icon == "" {
			icon = "?"
		}
		sb.WriteString(fmt.Sprintf("  %s [%s] %s: %s\n", icon, item.ID, item.Status, item.Content))
	}
	return sb.String()
}

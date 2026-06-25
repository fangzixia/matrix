package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// TodoItem 表示单条任务项。
type TodoItem struct {
	// ID 为 任务项唯一标识；merge=true 时用于匹配更新。
	ID string `json:"id"`
	// Content 为任务项描述内容。
	Content string `json:"content"`
	// Status 任务状态：pending / in_progress / completed / cancelled。
	Status string `json:"status"`
}

// NewTodoWriteTool 创建任务项写入工具。
//
// 职责边界：管理结构化任务列表，与 [NewSleepTool] 等工具互补。
//
// TodoWriteTool：
//   - merge=false 全量替换；merge=true 按 id 合并
//   - 持久化到沙箱 .matrix/todos.json
func NewTodoWriteTool() *Tool {
	return &Tool{
		Name: "todo_write",
		Description: `创建或更新结构化 TODO 列表，用于任务计划与进度跟踪。
使用方式：
- merge=false：用新 todos 完全替换现有列表
- merge=true：按 id 合并更新；相同 id 覆盖，新 id 追加

每个 todo 项须**同时**包含 id、content、status 三个字段（与 API 调用 JSON 结构一致）。
status 取值：pending、in_progress、completed、cancelled`,
		Parameters: JSONSchema{
			Type: "object",
			Properties: map[string]PropSchema{
				"todos": {
					Type:        "array",
					Description: "TODO 项列表，每项含 id、content、status",
					Items: &PropSchema{
						Type: "object",
						Properties: map[string]PropSchema{
							"id":      {Type: "string", Description: "唯一标识，merge 时用于匹配"},
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
					Description: "true 按 id 合并；false 全量替换（默认 false）",
				},
			},
			Required: []string{"todos"},
		},
		IsConcurrencySafe: false,
		Execute:           execTodoWrite,
	}
}

// todoWriteFilePath 返回任务项持久化文件路径。
func todoWriteFilePath(ctx context.Context) (string, error) {
	root := SandboxFrom(ctx)
	if root == "" {
		return "", fmt.Errorf("沙箱未配置")
	}
	return filepath.Join(root, ".matrix", "todos.json"), nil
}

// decodeTodos 从 JSON 解码任务项列表。
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
			return nil, fmt.Errorf("todos 类型 %T 无法解析", raw)
		}
		var items []TodoItem
		if err := json.Unmarshal(data, &items); err != nil {
			return nil, err
		}
		return items, nil
	}
}

// execTodoWrite 执行 todo_write 工具逻辑。
func execTodoWrite(ctx context.Context, args map[string]any) (string, error) {
	raw, ok := args["todos"]
	if !ok {
		return "", fmt.Errorf("todo_write: 缺少参数 todos")
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
			return "", fmt.Errorf("todo_write: 无效 status %q", item.Status)
		}
	}
	var finalItems []TodoItem
	if merge {
		existing := todoWriteLoad(ctx)
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
	todoPath, err := todoWriteFilePath(ctx)
	if err != nil {
		return "", fmt.Errorf("todo_write: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(todoPath), 0o755); err != nil {
		return "", fmt.Errorf("todo_write: 创建目录失败: %w", err)
	}
	if err := todoWriteSave(ctx, finalItems); err != nil {
		return "", fmt.Errorf("todo_write: 保存失败: %w", err)
	}
	text := todoWriteFormat(finalItems)
	EmitOutput(ctx, text+"\n")
	return text, nil
}

// todoWriteLoad 从文件加载任务项列表。
func todoWriteLoad(ctx context.Context) []TodoItem {
	path, err := todoWriteFilePath(ctx)
	if err != nil {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var items []TodoItem
	if err := json.Unmarshal(data, &items); err != nil {
		return nil
	}
	return items
}

// todoWriteSave 将任务项列表保存到文件。
func todoWriteSave(ctx context.Context, items []TodoItem) error {
	path, err := todoWriteFilePath(ctx)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// todoWriteFormat 格式化任务项列表为可读文本。
func todoWriteFormat(items []TodoItem) string {
	if len(items) == 0 {
		return "TODO 列表为空"
	}
	statusIcon := map[string]string{
		"pending": "○", "in_progress": "◐",
		"completed": "✓", "cancelled": "✗",
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("TODO 列表共 %d 项:\n", len(items)))
	for _, item := range items {
		icon := statusIcon[item.Status]
		if icon == "" {
			icon = "·"
		}
		sb.WriteString(fmt.Sprintf("  %s [%s] %s: %s\n", icon, item.ID, item.Status, item.Content))
	}
	return sb.String()
}

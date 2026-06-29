package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestTodoWriteIsolatedPerRunMatrixDir(t *testing.T) {
	base := t.TempDir()
	ctxA := WithMatrixDir(context.Background(), filepath.Join(base, "run-a"))
	ctxB := WithMatrixDir(context.Background(), filepath.Join(base, "run-b"))

	tool := NewTodoWriteTool()
	_, err := tool.Execute(ctxA, map[string]any{
		"todos": []any{
			map[string]any{"id": "1", "content": "task A", "status": "pending"},
		},
	})
	if err != nil {
		t.Fatalf("run A todo_write: %v", err)
	}
	_, err = tool.Execute(ctxB, map[string]any{
		"todos": []any{
			map[string]any{"id": "1", "content": "task B", "status": "in_progress"},
		},
	})
	if err != nil {
		t.Fatalf("run B todo_write: %v", err)
	}

	itemsA := readTodosFile(t, ctxA)
	itemsB := readTodosFile(t, ctxB)
	if len(itemsA) != 1 || itemsA[0].Content != "task A" {
		t.Fatalf("run A todos: %+v", itemsA)
	}
	if len(itemsB) != 1 || itemsB[0].Content != "task B" {
		t.Fatalf("run B todos: %+v", itemsB)
	}
}

func readTodosFile(t *testing.T, ctx context.Context) []TodoItem {
	t.Helper()
	path, err := todoWriteFilePath(ctx)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var items []TodoItem
	if err := json.Unmarshal(data, &items); err != nil {
		t.Fatal(err)
	}
	return items
}

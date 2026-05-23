package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"matrix/internal/matrixpaths"
)

func TestFileEditTool_Create(t *testing.T) {
	dir := t.TempDir()
	SetWorkspaceRoot(dir)
	tool := NewFileEditTool()
	path := filepath.Join(dir, "new.txt")

	out, err := tool.Execute(context.Background(), map[string]any{
		"file_path":  path,
		"old_string": "",
		"new_string": "hello world",
	})
	if err != nil {
		t.Fatalf("创建文件失败: %v", err)
	}
	if got, _ := os.ReadFile(path); string(got) != "hello world" {
		t.Errorf("文件内容错误: %q", got)
	}
	t.Log(out)
}

func TestFileEditTool_Replace(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "edit.txt")
	_ = os.WriteFile(path, []byte("foo bar foo"), 0o644)

	tool := NewFileEditTool()

	// 单次替换
	_, err := tool.Execute(context.Background(), map[string]any{
		"file_path":  path,
		"old_string": "foo",
		"new_string": "baz",
	})
	if err == nil {
		t.Fatal("多处匹配且 replace_all=false 应报错")
	}

	// replace_all
	_, err = tool.Execute(context.Background(), map[string]any{
		"file_path":   path,
		"old_string":  "foo",
		"new_string":  "baz",
		"replace_all": true,
	})
	if err != nil {
		t.Fatalf("replace_all 替换失败: %v", err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != "baz bar baz" {
		t.Errorf("replace_all 结果错误: %q", got)
	}
}

func TestFileEditTool_NotFound(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	_ = os.WriteFile(path, []byte("hello"), 0o644)

	tool := NewFileEditTool()
	_, err := tool.Execute(context.Background(), map[string]any{
		"file_path":  path,
		"old_string": "NOTEXIST",
		"new_string": "x",
	})
	if err == nil {
		t.Fatal("old_string 不存在时应报错")
	}
}

func TestGlobTool(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "a.go"), []byte(""), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "b.go"), []byte(""), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "c.txt"), []byte(""), 0o644)

	tool := NewGlobTool()
	out, err := tool.Execute(context.Background(), map[string]any{
		"pattern": "*.go",
		"path":    dir,
	})
	if err != nil {
		t.Fatalf("glob 失败: %v", err)
	}
	// 路径可能是绝对路径或相对路径，只检查文件名
	if !containsStr(out, "a.go") && !containsStr(out, "a") {
		t.Errorf("glob 结果缺少 a.go: %s", out)
	}
	if containsStr(out, "c.txt") {
		t.Errorf("glob 不应包含 .txt 文件: %s", out)
	}
}

func TestGrepTool(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "a.go"), []byte("func Hello() {}\nfunc World() {}"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "b.go"), []byte("func Foo() {}"), 0o644)

	tool := NewGrepTool()
	out, err := tool.Execute(context.Background(), map[string]any{
		"pattern":     "func Hello",
		"path":        dir,
		"output_mode": "files_with_matches",
	})
	if err != nil {
		t.Fatalf("grep 失败: %v", err)
	}
	if !containsStr(out, "a.go") {
		t.Errorf("grep 结果应包含 a.go: %s", out)
	}
	if containsStr(out, "b.go") {
		t.Errorf("grep 不应包含 b.go: %s", out)
	}
}

func TestSleepTool(t *testing.T) {
	tool := NewSleepTool()
	out, err := tool.Execute(context.Background(), map[string]any{"duration_ms": float64(10)})
	if err != nil {
		t.Fatalf("sleep 失败: %v", err)
	}
	if out == "" {
		t.Error("sleep 应返回非空输出")
	}
}

func TestTodoWriteTool(t *testing.T) {
	dir := t.TempDir()
	matrixpaths.SetDataRootForTest(t.TempDir())
	t.Cleanup(func() { matrixpaths.SetDataRootForTest("") })
	SetWorkspaceRoot(dir)

	tool := NewTodoWriteTool()
	out, err := tool.Execute(context.Background(), map[string]any{
		"todos": `[{"id":"1","content":"任务一","status":"pending"},{"id":"2","content":"任务二","status":"in_progress"}]`,
		"merge": false,
	})
	if err != nil {
		t.Fatalf("todo_write 失败: %v", err)
	}
	if !containsStr(out, "任务一") {
		t.Errorf("输出缺少任务一: %s", out)
	}
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && stringContains(s, sub))
}

func stringContains(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

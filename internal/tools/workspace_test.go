package tools

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRequireNewFileInWorkspace(t *testing.T) {
	ws := t.TempDir()
	other := t.TempDir()
	SetWorkspaceRoot(ws)

	if err := requireNewFileInWorkspace(filepath.Join(ws, "a.txt")); err != nil {
		t.Fatalf("工作区内路径应通过: %v", err)
	}
	if err := requireNewFileInWorkspace(filepath.Join(other, "b.txt")); err == nil {
		t.Fatal("工作区外路径应拒绝")
	}
}

func TestWriteFile_CreateOutsideWorkspaceDenied(t *testing.T) {
	ws := t.TempDir()
	other := t.TempDir()
	SetWorkspaceRoot(ws)

	outPath := filepath.Join(other, "x.txt")
	_, err := WriteFile.Execute(t.Context(), map[string]any{
		"path":    outPath,
		"content": "hi",
	})
	if err == nil {
		t.Fatal("应在工作区外新建文件时失败")
	}
	if _, statErr := os.Stat(outPath); statErr == nil {
		t.Fatal("不应创建文件")
	}
}

func TestWriteFile_OverwriteOutsideWorkspaceAllowed(t *testing.T) {
	other := t.TempDir()
	SetWorkspaceRoot(t.TempDir())

	outPath := filepath.Join(other, "existing.txt")
	if err := os.WriteFile(outPath, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := WriteFile.Execute(t.Context(), map[string]any{
		"path":    outPath,
		"content": "new",
	})
	if err != nil {
		t.Fatalf("覆盖工作区外已有文件应允许: %v", err)
	}
	got, _ := os.ReadFile(outPath)
	if string(got) != "new" {
		t.Fatalf("内容错误: %q", got)
	}
}

func TestFileEdit_CreateOutsideWorkspaceDenied(t *testing.T) {
	ws := t.TempDir()
	other := t.TempDir()
	SetWorkspaceRoot(ws)

	outPath := filepath.Join(other, "new.txt")
	_, err := NewFileEditTool().Execute(t.Context(), map[string]any{
		"file_path":  outPath,
		"old_string": "",
		"new_string": "x",
	})
	if err == nil {
		t.Fatal("应在工作区外新建时失败")
	}
}

func TestFileEdit_ModifyOutsideWorkspaceAllowed(t *testing.T) {
	other := t.TempDir()
	SetWorkspaceRoot(t.TempDir())

	outPath := filepath.Join(other, "edit.txt")
	_ = os.WriteFile(outPath, []byte("hello"), 0o644)
	_, err := NewFileEditTool().Execute(t.Context(), map[string]any{
		"file_path":  outPath,
		"old_string": "hello",
		"new_string": "world",
	})
	if err != nil {
		t.Fatalf("修改工作区外已有文件应允许: %v", err)
	}
}

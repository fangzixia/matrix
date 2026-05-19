package tools

import (
	"context"
	"strings"
	"testing"

	"matrix/internal/llm"
)

// mockTool 创建用于测试的虚拟工具。
func mockTool(name string, safe bool, output string) *Tool {
	return &Tool{
		Name:              name,
		Description:       "mock: " + name,
		Parameters:        JSONSchema{Type: "object"},
		IsConcurrencySafe: safe,
		Execute: func(_ context.Context, _ map[string]any) (string, error) {
			return output, nil
		},
	}
}

// TestPartitionCalls 验证只读工具被合并为并行批次、写工具各自串行。
func TestPartitionCalls(t *testing.T) {
	reg := NewRegistry()
	reg.Register(mockTool("r1", true, ""))
	reg.Register(mockTool("r2", true, ""))
	reg.Register(mockTool("w1", false, ""))
	reg.Register(mockTool("r3", true, ""))

	calls := []llm.ToolCall{
		{ID: "1", Function: llm.ToolCallFunction{Name: "r1"}},
		{ID: "2", Function: llm.ToolCallFunction{Name: "r2"}},
		{ID: "3", Function: llm.ToolCallFunction{Name: "w1"}},
		{ID: "4", Function: llm.ToolCallFunction{Name: "r3"}},
	}

	batches := partitionCalls(calls, reg)
	// 期望：[r1,r2] 并行、[w1] 串行、[r3] 并行
	if len(batches) != 3 {
		t.Fatalf("期望 3 个批次，实际 %d", len(batches))
	}
	if !batches[0].parallel || len(batches[0].calls) != 2 {
		t.Errorf("批次[0]: 期望 2 个并行只读工具，实际 parallel=%v len=%d",
			batches[0].parallel, len(batches[0].calls))
	}
	if batches[1].parallel || len(batches[1].calls) != 1 {
		t.Errorf("批次[1]: 期望 1 个串行写工具，实际 parallel=%v len=%d",
			batches[1].parallel, len(batches[1].calls))
	}
	if !batches[2].parallel || len(batches[2].calls) != 1 {
		t.Errorf("批次[2]: 期望 1 个并行只读工具，实际 parallel=%v len=%d",
			batches[2].parallel, len(batches[2].calls))
	}
}

// TestRunToolsPermissionDenied 验证权限拒绝时返回错误结果。
func TestRunToolsPermissionDenied(t *testing.T) {
	reg := NewRegistry()
	reg.Register(mockTool("secret", true, "敏感数据"))

	calls := []llm.ToolCall{
		{ID: "1", Function: llm.ToolCallFunction{Name: "secret", Arguments: "{}"}},
	}

	results := RunTools(context.Background(), calls, reg, func(name string, _ map[string]any) bool {
		return false // 拒绝所有工具
	})

	if len(results) != 1 {
		t.Fatalf("期望 1 个结果")
	}
	if !results[0].IsError {
		t.Error("期望返回权限拒绝错误")
	}
	if !strings.Contains(results[0].Output, "permission denied") {
		t.Errorf("错误信息异常: %s", results[0].Output)
	}
}

// TestRunToolsUnknownTool 验证未知工具名称被优雅处理。
func TestRunToolsUnknownTool(t *testing.T) {
	reg := NewRegistry()
	calls := []llm.ToolCall{
		{ID: "1", Function: llm.ToolCallFunction{Name: "不存在的工具", Arguments: "{}"}},
	}
	results := RunTools(context.Background(), calls, reg, nil)
	if !results[0].IsError {
		t.Error("期望未知工具返回错误")
	}
	if !strings.Contains(results[0].Output, "unknown tool") {
		t.Errorf("错误信息异常: %s", results[0].Output)
	}
}

// TestRunToolsConcurrent 验证多个只读工具并发执行均成功。
func TestRunToolsConcurrent(t *testing.T) {
	reg := NewRegistry()
	reg.Register(mockTool("ta", true, "结果_a"))
	reg.Register(mockTool("tb", true, "结果_b"))
	reg.Register(mockTool("tc", true, "结果_c"))

	calls := []llm.ToolCall{
		{ID: "1", Function: llm.ToolCallFunction{Name: "ta", Arguments: "{}"}},
		{ID: "2", Function: llm.ToolCallFunction{Name: "tb", Arguments: "{}"}},
		{ID: "3", Function: llm.ToolCallFunction{Name: "tc", Arguments: "{}"}},
	}
	results := RunTools(context.Background(), calls, reg, nil)
	if len(results) != 3 {
		t.Fatalf("期望 3 个结果，实际 %d", len(results))
	}
	for _, r := range results {
		if r.IsError {
			t.Errorf("%s 意外返回错误: %s", r.ToolName, r.Output)
		}
	}
}

// TestParseArgs 验证各种 JSON 参数字符串的解析行为。
func TestParseArgs(t *testing.T) {
	cases := []struct {
		raw     string
		wantErr bool
		key     string
		val     string
	}{
		{`{"path":"go.mod"}`, false, "path", "go.mod"},
		{`{}`, false, "", ""},
		{``, false, "", ""},
		{`null`, false, "", ""},
		{`不是JSON`, true, "", ""},
	}
	for _, tc := range cases {
		m, err := parseArgs(tc.raw)
		if tc.wantErr {
			if err == nil {
				t.Errorf("parseArgs(%q): 期望解析错误", tc.raw)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseArgs(%q): 意外错误: %v", tc.raw, err)
			continue
		}
		if tc.key != "" {
			v, ok := getString(m, tc.key)
			if !ok || v != tc.val {
				t.Errorf("parseArgs(%q)[%q]: 期望 %q，实际 %q", tc.raw, tc.key, tc.val, v)
			}
		}
	}
}

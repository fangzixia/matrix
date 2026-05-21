package query

import (
	"context"
	"strings"
	"testing"

	"matrix/internal/llm"
	"matrix/internal/tools"
)

// TestBuildChatMessages 验证内部消息历史到 llm.ChatMessage 的转换逻辑。
func TestBuildChatMessages(t *testing.T) {
	history := []Message{
		{Role: RoleUser, Content: "你好"},
		{
			Role: RoleAssistant,
			ToolCalls: []llm.ToolCall{
				{ID: "c1", Function: llm.ToolCallFunction{Name: "read_file", Arguments: `{"path":"f"}`}},
			},
		},
		{Role: RoleTool, Content: "文件内容", ToolCallID: "c1", ToolName: "read_file"},
		{Role: RoleAssistant, Content: "完成"},
	}

	msgs := buildChatMessages("系统提示", history)

	// system + 4 条历史 = 5 条
	if len(msgs) != 5 {
		t.Fatalf("期望 5 条消息，实际 %d", len(msgs))
	}
	if msgs[0].Role != "system" || msgs[0].Content != "系统提示" {
		t.Errorf("msgs[0] 异常: %+v", msgs[0])
	}
	if msgs[1].Role != "user" || msgs[1].Content != "你好" {
		t.Errorf("msgs[1] 异常: %+v", msgs[1])
	}
	// 带 tool_calls 的 assistant 消息：content 应为空字符串
	if msgs[2].Role != "assistant" {
		t.Errorf("msgs[2] role 异常: %q", msgs[2].Role)
	}
	if len(msgs[2].ToolCalls) != 1 {
		t.Errorf("msgs[2] tool_calls 数量异常: %d", len(msgs[2].ToolCalls))
	}
	if msgs[3].Role != "tool" || msgs[3].ToolCallID != "c1" {
		t.Errorf("msgs[3] 异常: %+v", msgs[3])
	}
	if msgs[4].Role != "assistant" || msgs[4].Content != "完成" {
		t.Errorf("msgs[4] 异常: %+v", msgs[4])
	}
}

// TestObserve 验证工具结果被正确转换为 role=tool 消息。
func TestObserve(t *testing.T) {
	results := []tools.Result{
		{ToolCallID: "id1", ToolName: "read_file", Output: "内容", IsError: false},
		{ToolCallID: "id2", ToolName: "bash", Output: "退出异常", IsError: true},
	}
	msgs := observe(context.Background(), Config{}, 1, results)

	if len(msgs) != 2 {
		t.Fatalf("期望 2 条消息，实际 %d", len(msgs))
	}
	if msgs[0].Role != RoleTool {
		t.Errorf("msgs[0] role 异常: %s", msgs[0].Role)
	}
	if msgs[0].ToolCallID != "id1" || msgs[0].IsError {
		t.Errorf("msgs[0] 内容异常: %+v", msgs[0])
	}
	if !msgs[1].IsError {
		t.Errorf("msgs[1] 应标记为错误")
	}
}

// TestObserve_Truncate 验证 MaxToolResultRunes 对 tool 输出的截断。
func TestObserve_Truncate(t *testing.T) {
	long := strings.Repeat("字", 100)
	results := []tools.Result{
		{ToolCallID: "id1", ToolName: "x", Output: long},
	}
	msgs := observe(context.Background(), Config{MaxToolResultRunes: 20}, 1, results)
	if len(msgs) != 1 {
		t.Fatalf("期望 1 条消息")
	}
	if strings.Contains(msgs[0].Content, "…（工具输出已按 MaxToolResultRunes 截断）") == false {
		t.Errorf("截断标记缺失: len=%d", len([]rune(msgs[0].Content)))
	}
}

// TestReport_NilHook 验证 nil Hook 时直接通过。
func TestReport_NilHook(t *testing.T) {
	if report(nil, nil) != "" {
		t.Error("nil hook 应返回空字符串")
	}
}

// TestReport_BlockingHook 验证 Stop Hook 返回非空字符串时循环需继续。
func TestReport_BlockingHook(t *testing.T) {
	msgs := []Message{{Role: RoleUser, Content: "测试"}}
	result := report(msgs, func(_ []Message) string {
		return "检测到不安全内容"
	})
	if !strings.Contains(result, "不安全") {
		t.Errorf("期望阻塞消息，实际: %q", result)
	}
}

// TestTransitionStr 验证跃迁原因格式化函数。
func TestTransitionStr(t *testing.T) {
	if transitionStr(nil) != "initial" {
		t.Error("nil 跃迁应返回 'initial'")
	}
	r := TransitionNextTurn
	if transitionStr(&r) != "next_turn" {
		t.Error("next_turn 跃迁格式化失败")
	}
	r2 := TransitionStopHookBlocking
	if transitionStr(&r2) != "stop_hook_blocking" {
		t.Error("stop_hook_blocking 跃迁格式化失败")
	}
}

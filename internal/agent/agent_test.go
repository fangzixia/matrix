package agent

import (
	"fmt"
	"strings"
	"testing"

	"matrix/internal/query"
)

// TestNewID 验证生成的 ID 格式符合 "agent-{8位十六进制}" 规范。
func TestNewID(t *testing.T) {
	id := NewID()
	if !strings.HasPrefix(string(id), "agent-") {
		t.Errorf("ID 格式错误，期望前缀 'agent-'，实际: %q", id)
	}
	if len(id) != len("agent-")+8 {
		t.Errorf("ID 长度错误，期望 %d，实际 %d", len("agent-")+8, len(id))
	}

	// 多次调用应生成不同 ID
	ids := make(map[ID]struct{})
	for i := 0; i < 100; i++ {
		ids[NewID()] = struct{}{}
	}
	if len(ids) < 90 {
		t.Errorf("ID 碰撞率过高：100 次生成仅得到 %d 个唯一 ID", len(ids))
	}
}

// TestRegistry 验证注册、查找、更新操作的正确性。
func TestRegistry(t *testing.T) {
	r := NewRegistry()
	id := NewID()

	// 查找不存在的 ID 返回 nil
	if got := r.Get(id); got != nil {
		t.Errorf("空注册表查找应返回 nil，实际: %v", got)
	}

	// 注册后可查找
	r.Register(&Record{ID: id, Description: "测试任务", Status: StatusRunning})
	rec := r.Get(id)
	if rec == nil {
		t.Fatal("注册后查找应返回非 nil")
	}
	if rec.Status != StatusRunning {
		t.Errorf("Status 错误，期望 %q，实际 %q", StatusRunning, rec.Status)
	}

	// Update 更新字段
	ok := r.Update(id, func(r *Record) { r.Status = StatusCompleted })
	if !ok {
		t.Error("Update 应返回 true")
	}
	if r.Get(id).Status != StatusCompleted {
		t.Error("Update 后 Status 应为 Completed")
	}

	// 更新不存在的 ID 返回 false
	if r.Update(NewID(), func(r *Record) {}) {
		t.Error("不存在 ID 的 Update 应返回 false")
	}
}

// TestFormatResult 验证 <result> XML 的基本格式。
func TestFormatResult(t *testing.T) {
	id := ID("agent-abcd1234")
	result := query.Result{
		StopReason: query.StopCompleted,
		TurnCount:  3,
		Answer:     "任务完成，文件已写入。",
	}
	xml := FormatResult(id, "写入文件", result)

	for _, want := range []string{
		"<result>",
		"<agent_id>agent-abcd1234</agent_id>",
		"<status>completed</status>",
		"写入文件",
		"任务完成，文件已写入。",
		"</result>",
	} {
		if !strings.Contains(xml, want) {
			t.Errorf("FormatResult 输出缺少 %q\n实际:\n%s", want, xml)
		}
	}
}

// TestFormatResult_Error 验证失败结果的 XML 格式。
func TestFormatResult_Error(t *testing.T) {
	id := ID("agent-err00001")
	result := query.Result{
		StopReason: query.StopModelError,
		TurnCount:  1,
		Err:        fmt.Errorf("API 超时"),
	}
	xml := FormatResult(id, "失败任务", result)
	if !strings.Contains(xml, "failed:") {
		t.Errorf("失败结果应包含 'failed:'，实际:\n%s", xml)
	}
}

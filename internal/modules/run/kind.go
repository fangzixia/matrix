package run

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Kind 是 Run / 任务类型。
type Kind string

const (
	KindPlan      Kind = "plan"
	KindImplement Kind = "implement"
	KindVerify    Kind = "verify"
	KindBuild     Kind = "build"
	KindChat      Kind = "chat"
)

// ParseKind 解析 API 传入的任务类型。
func ParseKind(s string) (Kind, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", fmt.Errorf("kind 不能为空")
	}
	k := Kind(s)
	return k, nil
}

// UnmarshalJSON 在 JSON 反序列化时校验任务类型枚举。
func (k *Kind) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	parsed, err := ParseKind(s)
	if err != nil {
		return err
	}
	*k = parsed
	return nil
}

// MarshalJSON 将任务类型序列化为 JSON 字符串。
func (k *Kind) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(*k))
}

// String 返回任务类型字符串（用于 DB、JSON、日志）。
func (k *Kind) String() string {
	return string(*k)
}

// IsHarness 报告是否为流水线阶段（含 build 编排入口；build 执行时循环 implement/verify）。
func (k *Kind) IsHarness() bool {
	return *k == KindPlan || *k == KindImplement || *k == KindVerify || *k == KindBuild
}

// RequiresApprovedPlan 报告启动该阶段是否需要已批准的计划文件。
func RequiresApprovedPlan(kind *Kind) bool {
	return *kind == KindImplement || *kind == KindVerify || *kind == KindBuild
}

// RequiresPlanFile 报告启动该阶段时是否必须设置 file_path。
func RequiresPlanFile(kind *Kind) bool {
	return *kind == KindImplement || *kind == KindVerify || *kind == KindBuild
}

// shouldNotifyRun 判断 Run 状态变更是否应发送通知，也是 UI 阶段列表的 kind 白名单。
func shouldNotifyRun(kind string) bool {
	k := Kind(kind)
	return k.IsHarness()
}

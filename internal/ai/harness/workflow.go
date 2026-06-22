// Package harness 定义 Plan/Implement/Verify/Build/UI Scan 任务流与 Prompt 模板。
package harness

import "strings"

// Kind 是 Harness 任务流类型。
type Kind string

const (
	// KindPlan 是计划编写阶段。
	KindPlan Kind = "plan"
	// KindImplement 是代码实现阶段。
	KindImplement Kind = "implement"
	// KindVerify 是验证阶段。
	KindVerify Kind = "verify"
	// KindBuild 是构建阶段。
	KindBuild Kind = "build"
	// KindUIScan 是 UI 扫描阶段。
	KindUIScan Kind = "ui-scan"
)

// State 是任务流执行状态。
type State string

const (
	// StatePrepared 表示已准备、尚未执行。
	StatePrepared State = "prepared"
	// StateRunning 表示正在执行。
	StateRunning State = "running"
	// StateSucceeded 表示执行成功。
	StateSucceeded State = "succeeded"
	// StateFailed 表示执行失败。
	StateFailed State = "failed"
)

// CoordinatorExecutionNote 追加到任务 prompt，提醒父 Agent 勿直接调用 Worker 工具。
const CoordinatorExecutionNote = `
【Coordinator 执行说明】
你是 Coordinator：不要直接调用 read_file、grep、glob、write_file、bash、list_dir 等；并行调研请在同一轮发起多个 agent。`

// Workflow 描述任务流的显式状态、预期产物与恢复指引，独立于 Prompt 文本。
type Workflow struct {
	Kind              Kind
	State             State
	Preset            string
	UserInput         string
	FilePath          string
	FileLabel         string
	FileFallback      string
	DefaultTask       string
	ExpectedArtifacts []string
	Acceptance        []string
	Recovery          []string
}

// Prompt 根据任务流状态、产物与验收标准生成完整 Prompt 文本。
func (w Workflow) Prompt() string {
	var b strings.Builder
	b.WriteString(w.Preset)
	if len(w.ExpectedArtifacts) > 0 || len(w.Acceptance) > 0 || len(w.Recovery) > 0 {
		b.WriteString("\n\n【任务状态】\n")
		b.WriteString("kind: ")
		b.WriteString(string(w.Kind))
		b.WriteString("\nstate: ")
		b.WriteString(string(w.State))
		writeList(&b, "expected_artifacts", w.ExpectedArtifacts)
		writeList(&b, "acceptance", w.Acceptance)
		writeList(&b, "recovery", w.Recovery)
	}
	if w.FileLabel != "" {
		b.WriteString("\n\n")
		b.WriteString(filePart(w.FileLabel, w.FilePath, w.FileFallback))
	}
	b.WriteString("\n\n任务描述: ")
	b.WriteString(userPart(w.UserInput, w.DefaultTask))
	b.WriteString(CoordinatorExecutionNote)
	return b.String()
}

// WithState 返回指定执行状态的任务流副本。
func (w Workflow) WithState(state State) Workflow {
	w.State = state
	return w
}

func writeList(b *strings.Builder, label string, values []string) {
	if len(values) == 0 {
		return
	}
	b.WriteString("\n")
	b.WriteString(label)
	b.WriteString(":")
	for _, value := range values {
		b.WriteString("\n- ")
		b.WriteString(value)
	}
}

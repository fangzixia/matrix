package tasks

import "strings"

type Kind string

const (
	KindSpec      Kind = "spec"
	KindImplement Kind = "implement"
	KindVerify    Kind = "verify"
	KindBuild     Kind = "build"
	KindUIScan    Kind = "ui-scan"
)

type State string

const (
	StatePrepared  State = "prepared"
	StateRunning   State = "running"
	StateSucceeded State = "succeeded"
	StateFailed    State = "failed"
)

// CoordinatorExecutionNote 追加到任务 prompt，提醒父 Agent 勿直接调用 Worker 工具。
const CoordinatorExecutionNote = `
【Coordinator 执行说明】
你是 Coordinator：不要直接调用 read_file、grep、glob、write_file、bash、list_dir 等；并行调研请在同一轮发起多个 agent。`

// Workflow makes task flows explicit enough for callers to track state,
// expected artifacts, and recovery guidance independently from the prompt.
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

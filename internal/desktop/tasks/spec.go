package tasks

import (
	"fmt"
	"strings"
	"time"
)

const specPresetBody = `【创建需求】
结合工作区项目源代码与 .matrix/ 下已有 SPEC-*.md（若有 SPEC-*.md 可作格式参考），将任务描述拆解为可验收的需求文档。

执行步骤：
1. 通过 agent 委派 Worker，使用 read_file/grep 调研相关源代码现状
2. 通过 agent 委派 Worker 列出并阅读 .matrix/ 下已有 SPEC-*.md 作为参考
3. 按下方文档结构编写内容；信息不足时在「待优化 / 待澄清」列出缺口，禁止编造未给出的指标
4. create 模式：通过 agent 委派 Worker 确保 .matrix/ 目录存在，并用 write_file 写入「目标需求文件」路径，不得另起文件名
5. update 模式：通过 agent 委派 Worker 先 read_file 目标文件，在保留合理既有结构前提下更新并写回

文档结构（Markdown）：
# {简短标题}

## 需求目标
（业务语言）

## 验收标准
- AC-1: …（可验证、可测试）
- AC-2: …

## 风险
- …

## 冲突与依赖
- …

## 待优化 / 待澄清
- …`

// resolveSpecTarget 无选中文件则生成新建路径，否则使用用户选中路径。
func resolveSpecTarget(filePath string, now time.Time) (target string, mode string) {
	if p := strings.TrimSpace(filePath); p != "" {
		return p, "update"
	}
	return fmt.Sprintf(".matrix/SPEC-%s.md", now.Format("20060102150405")), "create"
}

func buildSpecPreset(mode string) string {
	return specPresetBody + "\n操作模式: " + mode
}

func NewSpecWorkflow(userInput, filePath string) Workflow {
	return newSpecWorkflowAt(userInput, filePath, time.Now())
}

func newSpecWorkflowAt(userInput, filePath string, now time.Time) Workflow {
	target, mode := resolveSpecTarget(filePath, now)
	return Workflow{
		Kind:              KindSpec,
		State:             StatePrepared,
		Preset:            buildSpecPreset(mode),
		UserInput:         userInput,
		FilePath:          target,
		FileLabel:         "目标需求文件",
		DefaultTask:       "请根据我的描述编写需求文档。",
		ExpectedArtifacts: []string{target},
		Acceptance: []string{
			"需求目标使用业务语言描述",
			"每条验收标准以 AC-* 标识且可验证、可测试",
			"文档包含「风险」「冲突与依赖」「待优化 / 待澄清」章节",
			"内容基于源代码与历史 SPEC 分析，非凭空编造",
			"已保存到目标需求文件路径",
		},
		Recovery: []string{
			"信息不足时列出待澄清项，不要编造指标",
			"create 模式：.matrix 目录或写入失败时说明原因与建议",
			"update 模式：目标文件不存在时说明并询问是否改为新建",
		},
	}
}

// BuildSpecTask 组装「创建需求」任务的 prompt 正文（不含工作区前缀，由 Bridge.formatUserMessage 追加）。
func BuildSpecTask(userInput, filePath string) string {
	return NewSpecWorkflow(userInput, filePath).Prompt()
}

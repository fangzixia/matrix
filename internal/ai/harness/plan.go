package harness

import (
	"fmt"
	"strings"
	"time"
)

const planPresetBody = `【创建计划】
结合工作区项目源代码与 .matrix/ 下已有 PLAN-*.md（若有 PLAN-*.md 可作格式参考），将任务描述拆解为可验收的计划文档。

执行步骤：
1. 通过 agent 委派 Worker，使用 read_file/grep 调研相关源代码现状
2. 通过 agent 委派 Worker 列出并阅读 .matrix/ 下已有 PLAN-*.md 作为参考
3. 按下方文档结构编写内容；信息不足时在「待优化 / 待澄清」列出缺口，禁止编造未给出的指标
4. create 模式：通过 agent 委派 Worker 确保 .matrix/ 目录存在，并用 write_file 写入「目标计划文件」路径，不得另起文件
5. update 模式：通过 agent 委派 Worker 用 read_file 目标文件，在保留合理既有结构前提下更新并写回

文档结构（Markdown）：
# {简短标题}

## 计划目标
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

// resolvePlanTarget 无选中文件则生成新建路径，否则使用用户选中路径。
func resolvePlanTarget(filePath string, now time.Time) (target string, mode string) {
	if p := strings.TrimSpace(filePath); p != "" {
		return p, "update"
	}
	return fmt.Sprintf(".matrix/PLAN-%s.md", now.Format("20060102150405")), "create"
}

func buildPlanPreset(mode string) string {
	return planPresetBody + "\n操作模式: " + mode
}

// NewPlanWorkflow 创建「计划编写」流水线工作流。
func NewPlanWorkflow(userInput, filePath string) Workflow {
	return newPlanWorkflowAt(userInput, filePath, time.Now())
}

func newPlanWorkflowAt(userInput, filePath string, now time.Time) Workflow {
	target, mode := resolvePlanTarget(filePath, now)
	return Workflow{
		Kind:              KindPlan,
		State:             StatePrepared,
		Preset:            buildPlanPreset(mode),
		UserInput:         userInput,
		FilePath:          target,
		FileLabel:         "目标计划文件",
		DefaultTask:       "请根据我的描述编写计划文档。",
		ExpectedArtifacts: []string{target},
		Acceptance: []string{
			"计划目标使用业务语言描述",
			"每条验收标准以 AC-* 标识且可验证、可测试",
			"文档包含「风险」「冲突与依赖」「待优化 / 待澄清」章节",
			"内容基于源代码与历史 PLAN 分析，非凭空编造",
			"已保存到目标计划文件路径",
		},
		Recovery: []string{
			"信息不足时列出待澄清项，不要编造指标",
			"create 模式：.matrix 目录或写入失败时说明原因与建议",
			"update 模式：目标文件不存在时说明并询问是否改为新建",
		},
	}
}

// BuildPlanTask 组装「创建计划」任务的 prompt 正文（不含工作区前缀，由调用方追加）。
func BuildPlanTask(userInput, filePath string) string {
	return NewPlanWorkflow(userInput, filePath).Prompt()
}

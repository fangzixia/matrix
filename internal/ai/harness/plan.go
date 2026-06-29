package harness

import (
	"fmt"
	"matrix/internal/modules/workspace"
	"strings"
	"time"
)

const planPresetBody = `【创建计划】
以工作区项目源代码为第一信息源，结合用户任务描述，将需求拆解为可验收的计划文档。

执行步骤：
1. 【调研】通过 agent 委派 Worker 只读调研源代码（不修改文件），范围须覆盖：与用户任务相关的页面/流程、已有同类能力、明显缺口
2. 【综合】根据调研结果与用户描述，判断需求是否已部分满足、是否需新增能力、信息是否不足
3. 【落笔】按下方文档结构与写作要求编写内容；信息不足时在「待优化 / 待澄清」列出缺口，禁止编造未给出的指标
4. create 模式：通过 agent 委派 Worker 确保 docs/plans/ 目录存在，并用 write_file 写入「目标计划文件」绝对路径，不得另起文件
5. update 模式：通过 agent 委派 Worker 用 read_file 读取「目标计划文件」，在保留合理既有结构前提下更新并写回

调研汇报须包含（业务语言，不写实现细节）：
- 现状摘要：当前产品已具备什么、缺什么
- 影响范围：涉及哪些用户角色/页面/流程
- 与用户需求的关系：完全新增 / 增强 / 可能已部分满足

写作要求：
- 以用户可感知的产品视角撰写，描述用户能看到、能操作、能验证的体验与结果
- 非必要情况下不要出现编码、文件路径、类名、接口等技术实现细节
- 验收标准禁止模糊表述（如「优化体验」「完善功能」「尽量」「适当」）；能写成 AC 的内容不得放入「待优化 / 待澄清」
- 「风险」：写可能影响交付或体验的不确定因素，附简要缓解思路（业务语言）
- 「冲突与依赖」：写与其他需求、角色、流程的冲突或前置依赖
- 「待优化 / 待澄清」：仅写当前信息不足、须用户确认的点；无则写「无」

文档结构（Markdown）：
# {简短标题}

## 现状（基于源码调研）
- …

## 计划目标
（业务语言）

## 范围
### 本次包含
- …
### 本次不包含
- …
### 非目标
- …

## 验收标准
- AC-1 [P0]: {用户可观察的结果}
  - 验证：{用户可执行的操作步骤 + 期望看到的结果}
  - 边界：{可选，异常或边界情况}
- AC-2 [P1]: …
  - 验证：…

## 风险
- …

## 冲突与依赖
- …

## 待优化 / 待澄清
- …`

const planUpdateModeNote = `update 模式额外要求：
- 以「目标计划文件」+ 源码调研 + 用户新描述为准，不引用其他计划文件补全信息
- 对比目标计划既有 AC：保留仍有效的，修改已变化的，删除已过时
- 不随意改动用户已确认过的范围，除非用户描述明确要求`

// resolvePlanTarget 无选中文件则生成新建路径，否则使用用户选中路径。
func resolvePlanTarget(filePath string, now time.Time) (target string, mode string) {
	if p := strings.TrimSpace(filePath); p != "" {
		return p, "update"
	}
	return fmt.Sprintf("%s/PLAN-%s.md", workspace.DocsPlansRel, now.Format("20060102150405")), "create"
}

// buildPlanPreset 构建计划阶段的预设工作流配置。
func buildPlanPreset(mode string) string {
	s := planPresetBody + "\n操作模式: " + mode
	if mode == "update" {
		s += "\n\n" + planUpdateModeNote
	}
	return s
}

// NewPlanWorkflow 创建「计划编写」流水线工作流。
func NewPlanWorkflow(userInput, selectedPath, targetAbsPath string) Workflow {
	return newPlanWorkflowAt(userInput, selectedPath, targetAbsPath, time.Now())
}

// newPlanWorkflowAt 在指定路径创建计划工作流实例。
// selectedPath 为用户选中的计划逻辑路径，空表示 create；targetAbsPath 为写入用的绝对路径。
func newPlanWorkflowAt(userInput, selectedPath, targetAbsPath string, now time.Time) Workflow {
	_, mode := resolvePlanTarget(selectedPath, now)
	target := strings.TrimSpace(targetAbsPath)
	if target == "" {
		target, _ = resolvePlanTarget(selectedPath, now)
	}
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
			"内容基于源代码调研，非凭空编造；",
			"含「现状」章节，说明当前能力与缺口（业务语言）",
			"含「范围」章节，区分本次包含/不包含/非目标",
			"计划目标使用业务语言，面向用户可感知的产品视角",
			"每条 AC 有编号与优先级（P0/P1/P2），含可执行验证步骤，无模糊表述",
			"未把可写成 AC 的内容放入「待优化 / 待澄清」",
			"文档包含「风险」「冲突与依赖」「待优化 / 待澄清」章节",
			"非必要情况下不出现编码或技术实现细节",
			"已保存到目标计划文件路径",
		},
		Recovery: []string{
			"信息不足时列出待澄清项，不要编造指标",
			"create 模式：docs/plans 目录或写入失败时说明原因与建议",
			"update 模式：目标文件不存在时说明并询问是否改为新建",
		},
	}
}

// PlanTargetPath 返回计划文件逻辑路径（create 时自动生成）。
func PlanTargetPath(filePath string, now time.Time) string {
	target, _ := resolvePlanTarget(filePath, now)
	return target
}

// BuildPlanTask 组装「创建计划」任务的 prompt 正文（不含工作区前缀，由调用方追加）。
// selectedPath 为用户选中的计划逻辑路径，空表示 create；targetAbsPath 为写入用的绝对路径。
func BuildPlanTask(userInput, selectedPath, targetAbsPath string) string {
	return NewPlanWorkflow(userInput, selectedPath, targetAbsPath).Prompt()
}

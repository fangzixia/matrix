package harness

import (
	"fmt"
	"strings"
	"time"
)

const planPresetBody = `【创建计划】
以系统用户视角将用户需求拆解为计划文档：第一信息源是「任务描述」（用户原始需求），第二信息源是源码调研（仅用于判断现状与缺口，不写入正文）。最终目标：保证用户的需求能够满足——批准后，系统用户能按正文完成用户想要的事。

系统用户：在本系统中登录/操作、完成业务的角色（如运营、管理员、访客），不是 Matrix 平台操作者，也不是开发者。

执行步骤：
1. 【理解需求】精读「任务描述」，明确用户要什么、为谁、期望的可感知结果
2. 【调研】通过 agent 委派 Worker 只读调研源代码（不修改文件），范围须覆盖：与用户任务相关的页面/流程、已有同类能力、明显缺口
3. 【综合】根据调研结果与用户描述，判断需求是否已部分满足、是否需新增能力、信息是否不足
4. 【落笔】按下方双轨文档结构与写作要求编写内容；落笔前自问：系统用户能否按正文完成用户想要的事？信息不足时在「待确认」列出缺口，禁止编造未给出的指标
5. create 模式：通过 agent 委派 Worker 确保 docs/plans/ 目录存在，并用 write_file 写入「目标计划文件」绝对路径，不得另起文件
6. update 模式：通过 agent 委派 Worker 用 read_file 读取「目标计划文件」，在保留合理既有结构前提下更新并写回

调研汇报须包含（业务语言，不写实现细节）：
- 现状摘要：当前产品已具备什么、缺什么
- 影响范围：涉及哪些用户角色/页面/流程
- 与用户需求的关系：完全新增 / 增强 / 可能已部分满足

写作要求（正文 — 系统用户视角）：
- 锚定用户任务描述，以系统用户可感知的产品视角撰写：描述在系统中能看到、能操作、能验证的体验与结果
- 正文不得出现文件路径、类名、接口名、源码目录等实现细节；调研过程不得写入正文（不写「基于源码调研」）
- 「用户验收（产品语言）」固定句式：「作为 {系统用户角色}，当 {在系统中的操作/条件}，应 {可观察的业务结果}」，不用 AC 编号
- 「待确认」仅写须用户拍板的业务问题（优先级、范围取舍、指标口径）；能写成场景或 AC 的内容不得放入
- 「风险」：写可能影响交付或体验的不确定因素，附简要缓解思路（业务语言）
- 「冲突与依赖」：写与其他需求、角色、流程的冲突或前置依赖；无则写「无」

写作要求（附录 — 供 implement/verify 使用）：
- 「附录：技术验收标准」须与正文「用户验收」一一对应：每个 P0 场景至少映射一条 AC
- 每条 AC 有编号、优先级（P0/P1/P2）与验证通道标签 [UI] / [API] / [CLI] / [Test]（纯库逻辑才用 Test）
- AC 正文写系统用户可感知结果，禁止写成「组件/接口/类实现正确」等开发视角
- 「验证」= 系统用户的实际操作路径：[UI] 写登录/进哪页/点什么/看到什么；[API] 写谁调用/什么接口/期望什么业务结果（不只 status code）；[CLI] 写命令与期望输出
- 禁止「检查 XX 组件/文件是否存在」类开发视角步骤；禁止模糊表述（如「优化体验」「完善功能」「尽量」「适当」）

向用户汇报（Coordinator 回复用户时）：
- 用产品语言总结：目标、范围、待确认项、计划是否已写好
- 禁止在回复中出现：文件路径、Worker/agent、read_file、AC 编号、P0/P1、源码目录
- 引导用户：「请预览计划文档并确认；有待确认项请先回复。」

文档结构（Markdown）：
# {简短标题 — 用户能看懂的一句话}

## 摘要
- 做什么、为谁、完成后用户会看到/能做什么（3–5 条，禁止技术术语）

## 当前情况
- 现在产品/流程里已经有什么、缺什么、用户痛点（业务语言）

## 计划目标
（业务语言）

## 范围
### 本次包含
- …
### 本次不包含
- …
### 不在本次考虑
- …

## 用户验收（产品语言）
- 场景 1：作为 {角色}，当 {操作/条件}，应 {可观察结果}
- 场景 2：…

## 风险
- …

## 冲突与依赖
- …

## 待确认
- …

---

## 附录：技术验收标准
> 供实现与自动评测使用；正文已覆盖的用户场景须在此逐条可测。

- AC-1 [P0] [UI]: {系统用户可观察结果，可与上文场景对应}
  - 验证：{系统用户操作步骤 + 期望业务结果}
  - 边界：{可选，异常或边界情况}
- AC-2 [P1] [API]: …
  - 验证：…`

const planUpdateModeNote = `update 模式额外要求：
- 以「目标计划文件」+ 源码调研 + 用户新描述为准，不引用其他计划文件补全信息
- 对比目标计划既有附录 AC：保留仍有效的，修改已变化的，删除已过时；同步更新正文用户验收场景
- 不随意改动用户已确认过的范围，除非用户描述明确要求`

// resolvePlanTarget 无选中文件则生成新建路径，否则使用用户选中路径。
func resolvePlanTarget(filePath string, now time.Time) (target string, mode string) {
	if p := strings.TrimSpace(filePath); p != "" {
		return p, "update"
	}
	return fmt.Sprintf("%s/PLAN-%s.md", docsPlansRel, now.Format("20060102150405")), "create"
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
			"内容基于源代码调研，非凭空编造",
			"计划正文与附录均锚定用户任务描述，以系统用户可感知结果表述",
			"正文含「摘要」「当前情况」「用户验收（产品语言）」章节，使用系统用户视角业务语言",
			"含「范围」章节，区分本次包含/不包含/不在本次考虑",
			"每个 P0 用户验收场景在附录有对应 AC（含验证通道标签与可执行验证步骤）",
			"未把可写成场景或 AC 的内容放入「待确认」",
			"文档包含「风险」「冲突与依赖」「待确认」章节",
			"正文非必要情况下不出现编码或技术实现细节",
			"已保存到目标计划文件路径",
		},
		Recovery: []string{
			"信息不足时列出待确认项，不要编造指标",
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

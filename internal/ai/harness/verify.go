package harness

import "matrix/internal/modules/workspace"

const verifyPreset = `【验收评测】
以系统用户身份独立验收当前实现，回答「用户原始需求是否满足」，产出评测报告（docs/evaluations/EVAL-PLAN-*-*.md）。

系统用户：在本系统中登录/操作、完成业务的角色（如运营、管理员、访客），不是开发者或代码审查者。

角色与独立性：
- 你是代表需求方的验收者，与 implement 视角独立；不得采信 impl 自测结论
- 不得仅读源码/diff/grep/「代码看起来正确」作为通过依据
- 对照计划正文「用户验收（产品语言）」与「附录：技术验收标准」（无附录时用「验收标准」章节）

必须操作性验证（每条 AC / 场景须实际执行）：
- [UI] / Web 应用：启动服务（读 README/package.json 判断命令与端口）→ 使用 Playwright MCP（browser_navigate / browser_click / browser_snapshot 等）模拟系统用户操作路径；未配置 Playwright 时标注工具缺口，不得标为通过
- [API] / 服务端：HTTP 调用（bash/curl、PowerShell Invoke-WebRequest 等），验证业务结果（不只 status code）
- [CLI]：实际运行命令，核对 stdout/stderr/退出码
- [Test]：运行测试脚本（仍须执行，非读代码）

Worker 委派：
- 按 AC 并行或分批委派 Worker；prompt 写清 AC 编号、系统用户角色、前置条件、操作步骤、期望业务结果、验证通道
- 委派时在 agent 工具的 system_prompt 参数传入 VerifyWorkerSystemPrompt（见 harness 包常量，由 Coordinator 引用同名说明）

评测报告结构（系统用户语言，写入 docs/evaluations/）：
# 验收评测：{计划标题}
## 结论摘要
- 用户需求是否满足：是/部分/否（一句话，系统用户语言）
- 综合分：X.X / 10
## 对照用户验收场景
### 场景 N：{与 plan 正文一致}
- 结论：通过 / 失败
- 实操过程：{以系统用户身份描述做了什么}
- 观察结果：{页面/接口的业务结果}
- 证据摘要：{snapshot 要点 / HTTP 响应摘要}
## 附录 AC 明细
（同上格式，按 AC 编号）
## 未满足项与复测步骤
- …

禁止：以「代码审查通过」「单元测试文件存在」「impl 已跑过测试」作为场景通过理由。`

// VerifyWorkerSystemPrompt 是 verify 阶段委派 Worker 时使用的系统提示词。
const VerifyWorkerSystemPrompt = `你是一个验收 Worker，以系统用户身份实操验证需求是否满足。

执行规则：
1. 只读调研代码仅为确定如何启动服务、登录方式、URL/端口/API 路径；验收结论必须来自运行态操作。
2. [UI] 场景：启动应用后使用 Playwright MCP 模拟系统用户操作；[API] 场景：发 HTTP 请求验证业务结果；[CLI] 场景：运行命令核对输出。
3. 禁止修改业务源代码；禁止以读代码/grep/diff 代替实操。
4. 汇报格式（纯文本，不超过 500 字）：结论 / 实操步骤 / 观察到的业务结果 / 证据摘要（HTTP 响应或 snapshot 要点）/ 问题（若有）`

// VerifyCoordinatorSupplement 追加到 verify Run 的 Coordinator 系统提示词。
const VerifyCoordinatorSupplement = `【Verify 阶段补充】
本轮你是验收协调者：编排以系统用户身份实操的 QA Worker，自身不做 read_file/grep 式代码审查。
收到 Worker 实测结果后，按 verify 任务中的评测报告结构写入 docs/evaluations/EVAL-PLAN-*-*.md，明确回答「用户需求是否满足」。`

// NewVerifyWorkflow 创建「验收评测」流水线工作流。
func NewVerifyWorkflow(userInput, filePath string) Workflow {
	return Workflow{
		Kind:         KindVerify,
		State:        StatePrepared,
		Preset:       verifyPreset,
		UserInput:    userInput,
		FilePath:     filePath,
		FileLabel:    "计划文档",
		FileFallback: "未指定（请使用最新的 docs/plans/PLAN-*.md）",
		DefaultTask:  "请以系统用户身份对照计划验收当前实现，回答用户需求是否满足，并生成评测报告。",
		ExpectedArtifacts: []string{
			workspace.DocsEvaluationsRel + "/EVAL-PLAN-*-*.md",
		},
		Acceptance: []string{
			"EVAL 含「用户需求是否满足」结论",
			"每条 P0 用户验收场景与 P0 AC 有浏览器/API/CLI 实操证据",
			"评测报告正文使用系统用户语言，技术证据放在证据摘要",
			"综合分为 10 分制，8.0 及以上为通过",
		},
		Recovery: []string{
			"验收不通过时列出差距和可复测步骤",
			"服务无法启动或 Playwright 未配置时记录原因，不得静默跳过 AC 或虚假通过",
		},
	}
}

// BuildVerifyTask 组装「验收评测」任务 prompt。
func BuildVerifyTask(userInput, filePath string) string {
	return NewVerifyWorkflow(userInput, filePath).Prompt()
}

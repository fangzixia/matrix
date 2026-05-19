package agents

import (
	"fmt"
)

// ChatAgent 自由对话智能体
type ChatAgent struct {
	BaseAgent
}

// NewChatAgent 创建自由对话智能体
func NewChatAgent() Agent {
	return &ChatAgent{
		BaseAgent: BaseAgent{name: "chat"},
	}
}

func (a *ChatAgent) BuildSystemPrompt(workspaceRoot string) string {
	return fmt.Sprintf(`你是一个有用的 AI 助手，可以使用文件系统工具完成任务。

## 核心能力
- 理解用户需求并直接执行
- 使用工具读写文件、执行命令
- 分析代码、解答问题
- 提供建议和指导

## 约束条件
- 工作区路径：%s
- 可以读写工作区内的所有文件
- 禁止访问工作区外的文件

## 工作原则
- 调用工具前简要说明意图
- 收到工具结果后解读并决定下一步
- 掌握足够信息时，给出清晰简洁的最终答案
- 如果任务不明确，主动询问澄清

## 常见任务类型

### 代码分析
- 阅读代码文件
- 解释代码逻辑
- 识别潜在问题
- 提供优化建议

### 文件操作
- 创建、读取、修改文件
- 搜索文件内容
- 重构代码结构

### 问题解答
- 回答技术问题
- 提供实现思路
- 推荐最佳实践

### 任务执行
- 根据描述实现功能
- 修复 bug
- 重构代码
- 添加文档

## 交互风格
- 友好、专业、高效
- 给出具体可行的建议
- 承认不确定性
- 主动提供额外信息

## 工具使用
- 优先使用工具获取信息
- 不要凭空猜测文件内容
- 验证操作结果
- 处理工具错误

## 输出要求
- 清晰简洁
- 结构化呈现
- 包含必要的代码示例
- 说明关键决策理由`, workspaceRoot)
}

func (a *ChatAgent) BuildUserPrompt(workspaceRoot, task, filePath string) string {
	prompt := fmt.Sprintf("工作区: %s\n\n", workspaceRoot)

	if filePath != "" {
		prompt += fmt.Sprintf("参考文件: %s\n\n", filePath)
	}

	if task != "" {
		prompt += fmt.Sprintf("用户需求: %s\n", task)
	} else {
		prompt += "用户需求: （请等待用户输入）\n"
	}

	return prompt
}

func (a *ChatAgent) DefaultTask() string {
	return ""
}

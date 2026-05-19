package agents

// Agent 智能体接口
type Agent interface {
	// Name 返回智能体名称
	Name() string

	// BuildSystemPrompt 构建系统提示词
	BuildSystemPrompt(workspaceRoot string) string

	// BuildUserPrompt 构建用户提示词
	BuildUserPrompt(workspaceRoot, task, filePath string) string

	// DefaultTask 返回默认任务描述
	DefaultTask() string
}

// BaseAgent 基础智能体实现
type BaseAgent struct {
	name string
}

func (a *BaseAgent) Name() string {
	return a.name
}

func (a *BaseAgent) DefaultTask() string {
	return ""
}

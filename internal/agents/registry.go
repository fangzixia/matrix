package agents

import "fmt"

// Registry Agent 注册表
type Registry struct {
	agents map[string]Agent
}

// NewRegistry 创建新的 Agent 注册表
func NewRegistry() *Registry {
	r := &Registry{
		agents: make(map[string]Agent),
	}

	// 注册所有 Agent
	r.Register(NewAnalysisAgent())
	r.Register(NewRequirementsAgent())
	r.Register(NewCodeAgent())
	r.Register(NewEvalAgent())
	r.Register(NewBuildAgent())
	r.Register(NewChatAgent())

	return r
}

// Register 注册 Agent
func (r *Registry) Register(agent Agent) {
	r.agents[agent.Name()] = agent
}

// Get 获取 Agent
func (r *Registry) Get(name string) (Agent, error) {
	agent, exists := r.agents[name]
	if !exists {
		return nil, fmt.Errorf("agent not found: %s", name)
	}
	return agent, nil
}

// List 列出所有 Agent 名称
func (r *Registry) List() []string {
	names := make([]string, 0, len(r.agents))
	for name := range r.agents {
		names = append(names, name)
	}
	return names
}

// Has 检查 Agent 是否存在
func (r *Registry) Has(name string) bool {
	_, exists := r.agents[name]
	return exists
}

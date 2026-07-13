package sdk

import "matrix/ai/coordinator"

// CoordinatorConfig 为多 Agent 编排配置。
type CoordinatorConfig = coordinator.Config

// CoordinatorQueryConfigOverrides 为 Coordinator 对 query.Config 的覆盖项。
type CoordinatorQueryConfigOverrides = coordinator.QueryConfigOverrides

// AsyncSupport 为异步子 Agent 通道与计数支持。
type AsyncSupport = coordinator.AsyncSupport

// RunControl 跟踪 Worker 取消函数。
type RunControl = coordinator.RunControl

// StreamHub 将 Worker 过程消息推到 UI。
type StreamHub = coordinator.StreamHub

var (
	// NewAsyncSupport 创建异步子 Agent 支持。
	NewAsyncSupport = coordinator.NewAsyncSupport
	// NewRunControl 创建 RunControl。
	NewRunControl = coordinator.NewRunControl
	// NewStreamHub 创建 StreamHub。
	NewStreamHub = coordinator.NewStreamHub
	// NewParentRegistry 创建父 Agent 工具注册表。
	NewParentRegistry = coordinator.NewParentRegistry
	// CloneWorkerRegistry 克隆 Worker 可用工具注册表。
	CloneWorkerRegistry = coordinator.CloneWorkerRegistry
	// QueryConfigFromCoordinator 由 Coordinator 配置派生 query.Config。
	QueryConfigFromCoordinator = coordinator.QueryConfigFromCoordinator
	// BuildParentSystemPrompt 构造父 Agent 系统提示词。
	BuildParentSystemPrompt = coordinator.BuildParentSystemPrompt
)

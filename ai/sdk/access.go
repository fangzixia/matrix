package sdk

import "matrix/ai/access"

// Policy 为文件访问策略（允许目录与 scratch）。
type Policy = access.Policy

var (
	// WithPolicy 将 Policy 注入 context。
	WithPolicy = access.WithPolicy
	// PolicyFrom 从 context 读取 Policy。
	PolicyFrom = access.PolicyFrom
	// NewPolicy 创建访问策略。
	NewPolicy = access.NewPolicy
	// ResolveAllowed 解析并校验路径是否在允许范围内。
	ResolveAllowed = access.ResolveAllowed
	// CheckAllowed 检查路径是否允许访问。
	CheckAllowed = access.CheckAllowed
	// ScratchDir 返回运行 scratch 目录。
	ScratchDir = access.ScratchDir
	// WorkDir 返回工作目录。
	WorkDir = access.WorkDir
	// ResolveAbsolute 将路径解析为绝对路径。
	ResolveAbsolute = access.ResolveAbsolute
)

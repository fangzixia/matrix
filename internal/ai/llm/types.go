// Package llm 提供 OpenAI 兼容的流式 HTTP 客户端，遵循
// /v1/chat/completions SSE 协议，可对接 OpenAI、Ollama 等任意兼容端点。
package llm

// ChatRequest 是 POST /v1/chat/completions 的请求体。
type ChatRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Tools       []Tool        `json:"tools,omitempty"`
	Stream      bool          `json:"stream"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Temperature *float64      `json:"temperature,omitempty"`
}

// ChatMessage 表示对话历史中的一轮消息。
type ChatMessage struct {
	// Role 为消息角色：system、user、assistant 或 tool。
	Role string `json:"role"`
	// Content 为消息内容，可以是字符串或 []ContentPart。
	Content any `json:"content"`
	// [compat:deepseek] ReasoningContent 为助手消息中的思考内容。
	// DeepSeek Reasoner 私有扩展：非 OpenAI 标准字段。
	// 若上一轮响应包含该字段，必须在下次请求时原样回传，否则 API 返回 400。
	// 对标准 OpenAI 端点无影响（该字段会被忽略）。
	ReasoningContent string     `json:"reasoning_content,omitempty"` // [compat:deepseek]
	ToolCalls        []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID       string     `json:"tool_call_id,omitempty"`
	Name             string     `json:"name,omitempty"`
}

// ContentPart 表示结构化内容块（目前仅支持 type="text"）。
type ContentPart struct {
	// Type 为内容块类型，当前固定为 "text"。
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// Tool 描述模型可以调用的一个函数。
type Tool struct {
	// Type 固定为 "function"。
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

// ToolFunction 是 Tool 中的函数定义，包含名称、描述和参数 Schema。
type ToolFunction struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	// Parameters 为符合 JSON Schema 规范的参数描述对象。
	Parameters map[string]any `json:"parameters"`
}

// ToolCall 是模型决定调用工具时在响应中返回的调用描述。
type ToolCall struct {
	// Index 是流式传输时该调用在 tool_calls 数组中的位置，用于跨事件累积。
	Index int    `json:"index"`
	ID    string `json:"id"`
	// Type 固定为 "function"。
	Type     string           `json:"type"`
	Function ToolCallFunction `json:"function"`
}

// ToolCallFunction 携带工具名称和完整的 JSON 参数字符串。
type ToolCallFunction struct {
	Name string `json:"name"`
	// Arguments 为 JSON 编码的参数字符串，流式传输时逐块追加。
	Arguments string `json:"arguments"`
}

// StreamChunk 是从 SSE "data: {...}" 行解析出的单个事件负载。
type StreamChunk struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
}

// Choice 包含单个候选结果的增量内容和结束原因。
type Choice struct {
	Index int   `json:"index"`
	Delta Delta `json:"delta"`
	// FinishReason 为结束原因：空字符串、"stop" 或 "tool_calls"。
	FinishReason string `json:"finish_reason"`
}

// Delta 携带一次 SSE 事件中生成的增量内容。
type Delta struct {
	Role string `json:"role,omitempty"`
	// Content 为文本增量 token。
	Content string `json:"content,omitempty"`
	// [compat:claude] Thinking 为扩展思考增量，Claude Extended Thinking 私有扩展。
	Thinking string `json:"thinking,omitempty"` // [compat:claude]
	// [compat:deepseek] ReasoningContent 为思考增量，DeepSeek Reasoner 私有扩展，与 Thinking 语义相同。
	ReasoningContent string     `json:"reasoning_content,omitempty"` // [compat:deepseek]
	ToolCalls        []ToolCall `json:"tool_calls,omitempty"`
}

// AssistantTurn 是一次模型调用流式结束后完整组装的响应。
type AssistantTurn struct {
	// Content 为模型输出的完整文本内容。
	Content string
	// Thinking 为模型输出的完整思考内容。
	// [compat:claude] claude 通过 delta.thinking 输出；
	// [compat:deepseek] deepseek 通过 delta.reasoning_content 输出；
	// 两者在此统一归并，不支持思考的模型此字段为空字符串。
	Thinking string
	// ToolCalls 为模型请求调用的所有工具，已按 index 顺序完整组装。
	ToolCalls []ToolCall
	// FinishReason 为结束原因，如 "stop"、"tool_calls"、"length" 等。
	FinishReason string
}

// StreamEvent 是 [Client.Stream] 返回 channel 中的单个事件。
//
// 每个事件至多设置一个有效字段：TextDelta、ThinkingDelta 或 Turn/Err。
type StreamEvent struct {
	// TextDelta 为本次事件的文本增量 token，可能为空。
	TextDelta string
	// ThinkingDelta 为本次事件的思考增量 token，可能为空。
	ThinkingDelta string
	// Turn 在流正常结束时设置，携带完整的 AssistantTurn。
	Turn *AssistantTurn
	// Err 在发生错误时非 nil，此后 channel 关闭。
	Err error
}

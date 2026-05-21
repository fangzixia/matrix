package llm

import (
	"context"
	"fmt"
)

// Complete 通过流式对话补全收束为完整文本，返回助手内容。
// 用于会话摘要等无需工具调用的单次生成场景。
func (c *Client) Complete(ctx context.Context, req ChatRequest) (string, error) {
	var finalTurn *AssistantTurn
	for ev := range c.Stream(ctx, req) {
		if ev.Err != nil {
			return "", ev.Err
		}
		if ev.Turn != nil {
			finalTurn = ev.Turn
		}
	}
	if finalTurn == nil {
		return "", fmt.Errorf("llm: 流结束但未收到完整 turn")
	}
	text := finalTurn.Content
	if text == "" {
		text = finalTurn.Thinking
	}
	if text == "" {
		return "", fmt.Errorf("llm: 响应内容为空")
	}
	return text, nil
}

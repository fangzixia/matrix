package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client 是无外部依赖的 OpenAI 兼容对话客户端，支持 SSE 流式传输。
type Client struct {
	// BaseURL 为端点根地址，如 "https://api.openai.com" 或 "http://localhost:11434"。
	BaseURL string
	// APIKey 为 Bearer Token；本地无鉴权端点可留空。
	APIKey string
	// HTTPClient 可替换为自定义实现，常用于测试。
	HTTPClient *http.Client
}

// NewClient 创建一个使用合理默认值的 Client。
func NewClient(baseURL, apiKey string) *Client {
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		APIKey:  apiKey,
		HTTPClient: &http.Client{
			Timeout: 10 * time.Minute,
		},
	}
}

// Stream 发送流式对话补全请求，将 [StreamEvent] 写入返回的只读 channel。
// channel 在流正常结束或发生错误时关闭。
//
// 最后一个事件携带 Turn（成功）或 Err（失败）；
// 中间事件携带 TextDelta 或 ThinkingDelta 增量 token。
func (c *Client) Stream(ctx context.Context, req ChatRequest) <-chan StreamEvent {
	ch := make(chan StreamEvent, 16)
	req.Stream = true
	go func() {
		defer close(ch)
		if err := c.stream(ctx, req, ch); err != nil {
			// 直接发送错误事件；缓冲区（16）足够容纳此最终事件，不会阻塞。
			// 不用 select+ctx.Done() 以防止两分支同时就绪时随机丢弃错误。
			ch <- StreamEvent{Err: err}
		}
	}()
	return ch
}

// Context 通过流式对话补全收束为完整文本，返回助手内容。
// 用于会话摘要等无需工具调用的单次生成场景。
func (c *Client) Context(ctx context.Context, req ChatRequest) (string, error) {
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

// stream 为 Stream 的内部阻塞实现，负责 HTTP 请求和 SSE 解析。
func (c *Client) stream(ctx context.Context, req ChatRequest, ch chan<- StreamEvent) error {
	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("llm: 序列化请求失败: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.BaseURL+"/v1/chat/completions",
		bytes.NewReader(body),
	)
	if err != nil {
		return fmt.Errorf("llm: 构建 HTTP 请求失败: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	if c.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("llm: HTTP 请求失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("llm: 服务端返回 %d: %s", resp.StatusCode, raw)
	}
	return c.parseSSE(ctx, resp.Body, ch)
}

// parseSSE 逐行读取 Server-Sent Events 流并将解析结果写入 ch。
//
// SSE 格式示例：
//
//	data: {"choices":[{"delta":{"content":"你好"},"finish_reason":null}]}
//	data: [DONE]
func (c *Client) parseSSE(ctx context.Context, r io.Reader, ch chan<- StreamEvent) error {
	var (
		contentBuf  strings.Builder
		thinkingBuf strings.Builder
		// tcMap 按 index 稀疏存储 tool_call 累积状态，顺序无关。
		tcMap        = make(map[int]*ToolCall)
		finishReason string
	)

	// emit 向 ch 非阻塞发送事件，ctx 取消时返回 false。
	emit := func(ev StreamEvent) bool {
		select {
		case ch <- ev:
			return true
		case <-ctx.Done():
			return false
		}
	}
	scanner := bufio.NewScanner(r)
	// 增大扫描缓冲区，以应对包含大型 tool_call JSON 的行。
	scanner.Buffer(make([]byte, 1<<20), 1<<20)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue // SSE 事件分隔空行
		}
		if !strings.HasPrefix(line, "data: ") {
			continue // 跳过 "event:"、"id:"、"retry:" 等控制行
		}
		payload := strings.TrimPrefix(line, "data: ")
		if payload == "[DONE]" {
			turn := c.assembleTurn(&contentBuf, &thinkingBuf, tcMap, finishReason)
			emit(StreamEvent{Turn: turn})
			return nil
		}
		var chunk StreamChunk
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			continue // 格式异常的块跳过，不中断整个流
		}
		for _, choice := range chunk.Choices {
			if choice.FinishReason != "" {
				finishReason = choice.FinishReason
			}
			d := choice.Delta
			if d.Content != "" {
				contentBuf.WriteString(d.Content)
				if !emit(StreamEvent{TextDelta: d.Content}) {
					return ctx.Err()
				}
			}

			// [compat:claude]   delta.thinking          → 思考增量
			// [compat:deepseek] delta.reasoning_content → 思考增量（语义相同，字段名不同）
			// 两者均非 OpenAI 标准字段，此处统一归并处理。
			thinkingDelta := d.Thinking
			if thinkingDelta == "" {
				thinkingDelta = d.ReasoningContent
			}
			if thinkingDelta != "" {
				thinkingBuf.WriteString(thinkingDelta)
				if !emit(StreamEvent{ThinkingDelta: thinkingDelta}) {
					return ctx.Err()
				}
			}

			// tool_call 按 index 累积，流式分片的 arguments 逐块追加。
			for _, tc := range d.ToolCalls {
				acc, exists := tcMap[tc.Index]
				if !exists {
					acc = &ToolCall{Type: "function"}
					tcMap[tc.Index] = acc
				}
				if tc.ID != "" {
					acc.ID = tc.ID
				}
				if tc.Function.Name != "" {
					acc.Function.Name = tc.Function.Name
				}
				acc.Function.Arguments += tc.Function.Arguments
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("llm: 读取响应流失败: %w", err)
	}

	// 流在 [DONE] 前结束（截断或非标准实现），仍发出已收集内容。
	turn := c.assembleTurn(&contentBuf, &thinkingBuf, tcMap, finishReason)
	emit(StreamEvent{Turn: turn})
	return nil
}

// assembleTurn 将累积的缓冲区和 tool_calls map 组装为 AssistantTurn。
func (c *Client) assembleTurn(
	contentBuf, thinkingBuf *strings.Builder,
	tcMap map[int]*ToolCall,
	finishReason string,
) *AssistantTurn {
	turn := &AssistantTurn{
		Content:      contentBuf.String(),
		Thinking:     thinkingBuf.String(),
		FinishReason: finishReason,
	}
	for i := 0; ; i++ {
		tc, ok := tcMap[i]
		if !ok {
			break
		}
		turn.ToolCalls = append(turn.ToolCalls, *tc)
	}
	return turn
}

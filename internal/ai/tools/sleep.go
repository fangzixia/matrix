package tools

import (
	"context"
	"fmt"
	"time"
)

// NewSleepTool 创建等待指定时间的工具。
//
// SleepTool：
//   - 参数为 duration_ms（毫秒）
//   - 主要用于等待异步操作、轮询间隔等
//   - 最大等待 60 秒
func NewSleepTool() *Tool {
	return &Tool{
		Name:        "sleep",
		Description: "等待指定的毫秒数。用于等待异步操作、文件系统同步或重试前的延迟。最大 60000ms（60 秒）。",
		Parameters: JSONSchema{
			Type: "object",
			Properties: map[string]PropSchema{
				"duration_ms": {
					Type:        "integer",
					Description: "等待时间（毫秒），最大 60000",
				},
			},
			Required: []string{"duration_ms"},
		},
		IsConcurrencySafe: true,
		Execute: func(ctx context.Context, args map[string]any) (string, error) {
			ms, ok := args["duration_ms"].(float64)
			if !ok || ms <= 0 {
				return "", fmt.Errorf("sleep: duration_ms 必须为正整数")
			}
			if ms > 60000 {
				ms = 60000
			}
			d := time.Duration(ms) * time.Millisecond
			EmitStatus(ctx, fmt.Sprintf("等待 %.0fms …", ms))
			select {
			case <-time.After(d):
				return fmt.Sprintf("已等待 %.0fms", ms), nil
			case <-ctx.Done():
				return "", ctx.Err()
			}
		},
	}
}

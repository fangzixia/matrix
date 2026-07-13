package util

import (
	"encoding/json"
	"strings"
)

// parseArgs 将 OpenAI function.arguments（JSON 字符串）解码为 map[string]any。
// raw 为空字符串或 "null" 时返回空 map 而非错误。
func parseArgs(raw string) (map[string]any, error) {
	if raw == "" || raw == "null" {
		return map[string]any{}, nil
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return nil, err
	}
	return m, nil
}

// getString 从已解析的 args map 中按 key 提取字符串值。
// 键不存在或值不是 string 类型时返回 ("", false)。
func getString(args map[string]any, key string) (string, bool) {
	v, ok := args[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

// GetString 是 getString 的导出版本，供外部包（如 coordinator）使用。
func GetString(args map[string]any, key string) (string, bool) {
	return getString(args, key)
}

// GetBool 从 args 中读取布尔参数；兼容部分模型把布尔写成字符串或数字的情况。
func GetBool(args map[string]any, key string) bool {
	v, ok := args[key]
	if !ok || v == nil {
		return false
	}
	switch x := v.(type) {
	case bool:
		return x
	case string:
		s := strings.TrimSpace(strings.ToLower(x))
		return s == "true" || s == "1" || s == "yes"
	case float64:
		return x != 0
	default:
		return false
	}
}

package run

import "encoding/json"

// encodePipelineStages 将流水线阶段列表序列化为 JSON。
func encodePipelineStages(stages []string) string {
	if len(stages) == 0 {
		return "[]"
	}
	b, _ := json.Marshal(stages)
	return string(b)
}

// decodePipelineStages 从 JSON 反序列化流水线阶段列表。
func decodePipelineStages(raw string) []string {
	if raw == "" || raw == "null" {
		return nil
	}
	var stages []string
	if err := json.Unmarshal([]byte(raw), &stages); err != nil {
		return nil
	}
	return stages
}

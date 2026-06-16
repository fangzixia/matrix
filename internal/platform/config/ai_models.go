package config

// ModelProfile 系统级可配置的模型条目（支持多条启用/默认）。
type ModelProfile struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	BaseURL   string `json:"base_url"`
	APIKey    string `json:"api_key"`
	Model     string `json:"model"`
	MaxTokens int    `json:"max_tokens"`
	Enabled   bool   `json:"enabled"`
	Default   bool   `json:"default"`
}

func (p ModelProfile) ToYAML() ModelYAML {
	max := p.MaxTokens
	if max <= 0 {
		max = 8192
	}
	return ModelYAML{
		BaseURL:   p.BaseURL,
		APIKey:    p.APIKey,
		Model:     p.Model,
		MaxTokens: max,
	}
}

// ActiveModel 返回当前启用的默认模型；无多模型配置时回退到 DefaultModel。
func (a AIConfig) ActiveModel() ModelYAML {
	if len(a.Models) > 0 {
		for _, m := range a.Models {
			if m.Enabled && m.Default {
				return m.ToYAML()
			}
		}
		for _, m := range a.Models {
			if m.Enabled {
				return m.ToYAML()
			}
		}
	}
	return a.DefaultModel
}

// SyncDefaultModel 将 ActiveModel 写回 DefaultModel，供旧代码路径使用。
func SyncDefaultModel(a *AIConfig) {
	if a == nil {
		return
	}
	a.DefaultModel = a.ActiveModel()
}

// NormalizeModelProfiles 补全 ID 并保证仅有一个启用的默认模型。
func NormalizeModelProfiles(models []ModelProfile) []ModelProfile {
	if len(models) == 0 {
		return models
	}
	for i := range models {
		if models[i].MaxTokens <= 0 {
			models[i].MaxTokens = 8192
		}
	}
	enabledIdx := -1
	defaultIdx := -1
	for i, m := range models {
		if !m.Enabled {
			models[i].Default = false
			continue
		}
		if enabledIdx < 0 {
			enabledIdx = i
		}
		if m.Default {
			if defaultIdx < 0 {
				defaultIdx = i
			} else {
				models[i].Default = false
			}
		}
	}
	if defaultIdx < 0 && enabledIdx >= 0 {
		models[enabledIdx].Default = true
	}
	return models
}

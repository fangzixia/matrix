package settings

import (
	"matrix/internal/platform/config"
	"strings"

	"github.com/google/uuid"
)

// normalizeModelProfiles 补全 ID、默认 max_tokens 与显示名称。
func normalizeModelProfiles(models *[]ModelProfileSettings) {
	if models == nil {
		return
	}
	for i := range *models {
		if (*models)[i].ID == "" {
			(*models)[i].ID = uuid.NewString()
		}
		if (*models)[i].MaxTokens <= 0 {
			(*models)[i].MaxTokens = 8192
		}
		if strings.TrimSpace((*models)[i].Name) == "" {
			(*models)[i].Name = (*models)[i].Model
		}
		if (*models)[i].Name == "" {
			(*models)[i].Name = "未命名模型"
		}
		if !(*models)[i].Multimodal {
			(*models)[i].AttachmentTypes = nil
		}
	}
	profiles := toConfigModels(*models)
	profiles = config.NormalizeModelProfiles(profiles)
	*models = fromConfigModels(profiles)
}

// maskModelProfiles 对外响应时清空 api_key，仅保留 api_key_set 标记。
func maskModelProfiles(models *[]ModelProfileSettings) {
	if models == nil {
		return
	}
	for i := range *models {
		key := (*models)[i].APIKey
		(*models)[i].APIKeySet = key != "" || (*models)[i].APIKeySet
		(*models)[i].APIKey = ""
	}
}

// mergeModelAPIKeys 保存时若未提交新 Key，则沿用数据库中的旧值。
func mergeModelAPIKeys(out *[]ModelProfileSettings, existing []ModelProfileSettings) {
	if out == nil || len(existing) == 0 {
		return
	}
	byID := make(map[string]string, len(existing))
	for _, m := range existing {
		if m.APIKey != "" {
			byID[m.ID] = m.APIKey
		}
	}
	for i := range *out {
		if (*out)[i].APIKey == "" {
			if key, ok := byID[(*out)[i].ID]; ok {
				(*out)[i].APIKey = key
			}
		}
		if (*out)[i].APIKey != "" {
			(*out)[i].APIKeySet = true
		}
	}
}

// toConfigModels 将 ModelProfileSettings 转换为 config.ModelProfile。
func toConfigModels(in []ModelProfileSettings) []config.ModelProfile {
	out := make([]config.ModelProfile, 0, len(in))
	for _, m := range in {
		out = append(out, config.ModelProfile{
			ID: m.ID, Name: m.Name, BaseURL: m.BaseURL, APIKey: m.APIKey,
			Model: m.Model, MaxTokens: m.MaxTokens, Enabled: m.Enabled, Default: m.Default,
			Multimodal: m.Multimodal, AttachmentTypes: append([]string(nil), m.AttachmentTypes...),
		})
	}
	return out
}

// fromConfigModels 将 config.ModelProfile 转换为 ModelProfileSettings。
func fromConfigModels(in []config.ModelProfile) []ModelProfileSettings {
	if len(in) == 0 {
		return nil
	}
	out := make([]ModelProfileSettings, 0, len(in))
	for _, m := range in {
		out = append(out, ModelProfileSettings{
			ID: m.ID, Name: m.Name, BaseURL: m.BaseURL, APIKey: m.APIKey,
			APIKeySet: m.APIKey != "", Model: m.Model, MaxTokens: m.MaxTokens,
			Enabled: m.Enabled, Default: m.Default,
			Multimodal: m.Multimodal, AttachmentTypes: append([]string(nil), m.AttachmentTypes...),
		})
	}
	return out
}

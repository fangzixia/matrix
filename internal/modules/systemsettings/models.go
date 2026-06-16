package systemsettings

import (
	"strings"

	"github.com/google/uuid"

	"matrix/internal/platform/config"
)

func migrateLegacyModel(st *Settings) {
	if st == nil || len(st.Models) > 0 {
		return
	}
	leg := st.Model
	if leg.Model == "" && leg.BaseURL == "" && !leg.APIKeySet && leg.APIKey == "" {
		return
	}
	name := leg.Model
	if name == "" {
		name = "默认模型"
	}
	st.Models = []ModelProfileSettings{{
		ID:        "default",
		Name:      name,
		BaseURL:   leg.BaseURL,
		APIKey:    leg.APIKey,
		APIKeySet: leg.APIKeySet || leg.APIKey != "",
		Model:     leg.Model,
		MaxTokens: leg.MaxTokens,
		Enabled:   true,
		Default:   true,
	}}
}

func normalizeModelSettings(st *Settings) {
	if st == nil {
		return
	}
	migrateLegacyModel(st)
	for i := range st.Models {
		if st.Models[i].ID == "" {
			st.Models[i].ID = uuid.NewString()
		}
		if st.Models[i].MaxTokens <= 0 {
			st.Models[i].MaxTokens = 8192
		}
		if strings.TrimSpace(st.Models[i].Name) == "" {
			st.Models[i].Name = st.Models[i].Model
		}
		if st.Models[i].Name == "" {
			st.Models[i].Name = "未命名模型"
		}
	}
	profiles := toConfigModels(st.Models)
	profiles = config.NormalizeModelProfiles(profiles)
	st.Models = fromConfigModels(profiles, config.ModelYAML{})
}

func maskModelProfiles(st *Settings) {
	if st == nil {
		return
	}
	for i := range st.Models {
		key := st.Models[i].APIKey
		st.Models[i].APIKeySet = key != "" || st.Models[i].APIKeySet
		st.Models[i].APIKey = ""
	}
}

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

func toConfigModels(in []ModelProfileSettings) []config.ModelProfile {
	out := make([]config.ModelProfile, 0, len(in))
	for _, m := range in {
		out = append(out, config.ModelProfile{
			ID: m.ID, Name: m.Name, BaseURL: m.BaseURL, APIKey: m.APIKey,
			Model: m.Model, MaxTokens: m.MaxTokens, Enabled: m.Enabled, Default: m.Default,
		})
	}
	return out
}

func fromConfigModels(in []config.ModelProfile, fallback config.ModelYAML) []ModelProfileSettings {
	if len(in) == 0 {
		return nil
	}
	out := make([]ModelProfileSettings, 0, len(in))
	for _, m := range in {
		out = append(out, ModelProfileSettings{
			ID: m.ID, Name: m.Name, BaseURL: m.BaseURL, APIKey: m.APIKey,
			APIKeySet: m.APIKey != "", Model: m.Model, MaxTokens: m.MaxTokens,
			Enabled: m.Enabled, Default: m.Default,
		})
	}
	_ = fallback
	return out
}

func defaultModelFromYAML(y config.ModelYAML) ModelProfileSettings {
	name := y.Model
	if name == "" {
		name = "默认模型"
	}
	return ModelProfileSettings{
		ID: "default", Name: name, BaseURL: y.BaseURL, APIKey: y.APIKey,
		APIKeySet: y.APIKey != "", Model: y.Model, MaxTokens: y.MaxTokens,
		Enabled: true, Default: true,
	}
}

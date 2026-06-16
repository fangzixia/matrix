package config

import "testing"

func TestActiveModelPrefersDefaultEnabled(t *testing.T) {
	ai := AIConfig{
		DefaultModel: ModelYAML{Model: "legacy"},
		Models: []ModelProfile{
			{ID: "1", Model: "a", Enabled: true, Default: false, MaxTokens: 100},
			{ID: "2", Model: "b", Enabled: true, Default: true, MaxTokens: 100},
		},
	}
	got := ai.ActiveModel()
	if got.Model != "b" {
		t.Fatalf("got %q want b", got.Model)
	}
}

func TestNormalizeModelProfilesSingleDefault(t *testing.T) {
	models := NormalizeModelProfiles([]ModelProfile{
		{ID: "1", Enabled: true, Default: true, MaxTokens: 100},
		{ID: "2", Enabled: true, Default: true, MaxTokens: 100},
	})
	if models[0].Default && models[1].Default {
		t.Fatal("expected only one default")
	}
}

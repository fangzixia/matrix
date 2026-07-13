// 最小宿主示例：仅 import matrix/ai/sdk。
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"matrix/ai/sdk"
)

func main() {
	baseURL := envOr("LLM_BASE_URL", "http://localhost:11434/v1")
	apiKey := os.Getenv("LLM_API_KEY")
	model := envOr("LLM_MODEL", "qwen2.5-coder:7b")

	client := sdk.NewClient(baseURL, apiKey)
	reg := sdk.RegistryWithoutShell(nil)

	sink := sdk.FuncSink(func(_ context.Context, ev sdk.Event) error {
		fmt.Printf("[%s] %s\n", sdk.EventType(ev), ev)
		return nil
	})

	ctx := context.Background()
	ctx = sdk.WithPolicy(ctx, sdk.NewPolicy([]string{os.TempDir()}, ""))

	result := sdk.RunSession(ctx, sdk.Config{
		LLM:          client,
		Model:        model,
		ThreadID:     "demo-thread-1",
		RunID:        sdk.NewRunID(),
		Registry:     reg,
		MaxTurns:     3,
		SystemPrompt: "You are a helpful assistant. Reply briefly.",
		InitialMessages: []sdk.Message{
			{Role: sdk.RoleUser, Content: "Say hello in one sentence."},
		},
	}, sink)

	if result.Err != nil {
		log.Fatalf("run failed: %v (%s)", result.Err, result.StopReason)
	}
	fmt.Printf("\n--- done: %s turns=%d answer=%q\n", result.StopReason, result.TurnCount, result.Answer)
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

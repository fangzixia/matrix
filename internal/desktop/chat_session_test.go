package desktop

import (
	"os"
	"path/filepath"
	"testing"

	"matrix/internal/query"
)

func TestChatTurnsToMessages(t *testing.T) {
	msgs := chatTurnsToMessages([]ChatTurn{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi"},
		{Role: "bogus", Content: "skip"},
	})
	if len(msgs) != 2 {
		t.Fatalf("got %d messages", len(msgs))
	}
	if msgs[0].Role != query.RoleUser || msgs[1].Role != query.RoleAssistant {
		t.Fatalf("unexpected roles: %+v", msgs)
	}
}

func TestPersistedTranscriptRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "chat-1.json")
	orig := []query.Message{
		{Role: query.RoleUser, Content: "u1"},
		{Role: query.RoleAssistant, Content: "a1", Thinking: "think"},
		{Role: query.RoleTool, Content: "tool out", ToolCallID: "tc1", ToolName: "read"},
	}
	if err := savePersistedTranscript(path, orig); err != nil {
		t.Fatal(err)
	}
	loaded, err := loadPersistedTranscript(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != len(orig) {
		t.Fatalf("len %d want %d", len(loaded), len(orig))
	}
	if loaded[2].ToolCallID != "tc1" {
		t.Fatalf("tool message: %+v", loaded[2])
	}
}

func TestLoadChatTranscriptBootstrap(t *testing.T) {
	b := NewBridge(DefaultConfig())
	b.config.Workspace.Root = t.TempDir()
	id := "test-chat"
	msgs, err := b.loadChatTranscript(id, []ChatTurn{
		{Role: "user", Content: "first"},
		{Role: "assistant", Content: "second"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 2 {
		t.Fatalf("got %d", len(msgs))
	}
	// 写入磁盘后再加载应命中 cache/文件
	if err := b.saveChatTranscript(id, append(msgs, query.Message{Role: query.RoleUser, Content: "third"})); err != nil {
		t.Fatal(err)
	}
	path := persistedTranscriptPath(b.workspaceRoot(), id)
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	b.chatStore.delete(id)
	again, err := b.loadChatTranscript(id, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(again) != 3 {
		t.Fatalf("reload got %d", len(again))
	}
}

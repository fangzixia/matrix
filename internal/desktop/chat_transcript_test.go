package desktop

import (
	"os"
	"path/filepath"
	"testing"

	"matrix/internal/matrixpaths"
	"matrix/internal/query"
)

func useAppDataStore(t *testing.T) {
	matrixpaths.SetDataRootForTest(t.TempDir())
	t.Cleanup(func() { matrixpaths.SetDataRootForTest("") })
}

func TestChatTurnsToMessages(t *testing.T) {
	tests := []struct {
		name string
		in   []ChatTurn
		want []query.Message
	}{
		{
			name: "user and assistant",
			in: []ChatTurn{
				{Role: "user", Content: "hello"},
				{Role: "assistant", Content: "hi"},
			},
			want: []query.Message{
				{Role: query.RoleUser, Content: "hello"},
				{Role: query.RoleAssistant, Content: "hi"},
			},
		},
		{
			name: "skip unknown role and empty content",
			in: []ChatTurn{
				{Role: "user", Content: "hello"},
				{Role: "bogus", Content: "skip"},
				{Role: "assistant", Content: ""},
				{Role: "", Content: "anon user"},
			},
			want: []query.Message{
				{Role: query.RoleUser, Content: "hello"},
				{Role: query.RoleUser, Content: "anon user"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := chatTurnsToMessages(tt.in)
			if len(got) != len(tt.want) {
				t.Fatalf("len %d want %d: %+v", len(got), len(tt.want), got)
			}
			for i := range tt.want {
				if got[i].Role != tt.want[i].Role || got[i].Content != tt.want[i].Content {
					t.Fatalf("[%d] got %+v want %+v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestTranscriptFileRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "chat-1.json")
	orig := []query.Message{
		{Role: query.RoleUser, Content: "u1"},
		{Role: query.RoleAssistant, Content: "a1", Thinking: "think"},
		{Role: query.RoleTool, Content: "tool out", ToolCallID: "tc1", ToolName: "read"},
	}
	if err := writeTranscriptFile(path, orig); err != nil {
		t.Fatal(err)
	}
	loaded, err := readTranscriptFile(path)
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

func TestChatTranscriptStoreLoadBootstrap(t *testing.T) {
	useAppDataStore(t)
	dir := t.TempDir()
	rootFn := func() string { return dir }
	store := NewChatTranscriptStore(rootFn)
	chatSessionID := "test-chat"

	// 无 cache/磁盘时用 bootstrap（对话历史降级）恢复
	msgs, err := store.Load(chatSessionID, []ChatTurn{
		{Role: "user", Content: "first"},
		{Role: "assistant", Content: "second"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 2 {
		t.Fatalf("got %d", len(msgs))
	}

	if err := store.Save(chatSessionID, append(msgs, query.Message{Role: query.RoleUser, Content: "third"})); err != nil {
		t.Fatal(err)
	}
	path := store.transcriptPath(chatSessionID)
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	wantPath := filepath.Join(matrixpaths.ChatTranscriptsDir(dir), chatSessionID+".json")
	if path != wantPath {
		t.Fatalf("path %q want %q", path, wantPath)
	}

	// 新 Store 实例模拟进程内 cache miss，应从磁盘加载
	reloadStore := NewChatTranscriptStore(rootFn)
	again, err := reloadStore.Load(chatSessionID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(again) != 3 {
		t.Fatalf("reload got %d", len(again))
	}
}

func TestChatTranscriptStoreEmptyID(t *testing.T) {
	useAppDataStore(t)
	store := NewChatTranscriptStore(func() string { return t.TempDir() })

	if _, err := store.Load("", nil); err == nil {
		t.Fatal("Load empty id want error")
	}
	if err := store.Save("", nil); err != nil {
		t.Fatal(err)
	}
	if err := store.Clear(""); err != nil {
		t.Fatal(err)
	}
}

func TestChatTranscriptStoreInvalidateCache(t *testing.T) {
	useAppDataStore(t)
	dir := t.TempDir()
	store := NewChatTranscriptStore(func() string { return dir })
	id := "cached"
	if err := store.Save(id, []query.Message{{Role: query.RoleUser, Content: "x"}}); err != nil {
		t.Fatal(err)
	}
	store.InvalidateCache()
	// cache miss 后应从磁盘加载，而非返回空
	again, err := store.Load(id, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(again) != 1 || again[0].Content != "x" {
		t.Fatalf("got %+v", again)
	}
}

func TestChatTranscriptStoreWorkspaceIsolation(t *testing.T) {
	useAppDataStore(t)
	dirA := t.TempDir()
	dirB := t.TempDir()
	id := "same-id"
	storeA := NewChatTranscriptStore(func() string { return dirA })
	if err := storeA.Save(id, []query.Message{{Role: query.RoleUser, Content: "from A"}}); err != nil {
		t.Fatal(err)
	}
	storeB := NewChatTranscriptStore(func() string { return dirB })
	msgs, err := storeB.Load(id, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 0 {
		t.Fatalf("workspace B should not see A transcript: %+v", msgs)
	}
}

func TestSetWorkspaceInvalidatesTranscriptCache(t *testing.T) {
	useAppDataStore(t)
	dirA := t.TempDir()
	dirB := t.TempDir()
	b := NewBridge(DefaultConfig())
	b.config.Workspace.Root = dirA
	id := "shared"

	if err := b.chatTranscripts.Save(id, []query.Message{{Role: query.RoleUser, Content: "from A"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.chatTranscripts.Load(id, nil); err != nil {
		t.Fatal(err)
	}

	if err := b.SetWorkspace(dirB); err != nil {
		t.Fatal(err)
	}
	msgs, err := b.chatTranscripts.Load(id, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 0 {
		t.Fatalf("after switch to B, cache must not serve A transcript: %+v", msgs)
	}

	if err := b.chatTranscripts.Save(id, []query.Message{{Role: query.RoleUser, Content: "from B"}}); err != nil {
		t.Fatal(err)
	}
	if err := b.SetWorkspace(dirA); err != nil {
		t.Fatal(err)
	}
	again, err := b.chatTranscripts.Load(id, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(again) != 1 || again[0].Content != "from A" {
		t.Fatalf("switch back to A want disk transcript from A, got %+v", again)
	}
}

func TestChatTranscriptStoreClear(t *testing.T) {
	useAppDataStore(t)
	dir := t.TempDir()
	store := NewChatTranscriptStore(func() string { return dir })
	id := "clear-me"
	if err := store.Save(id, []query.Message{{Role: query.RoleUser, Content: "x"}}); err != nil {
		t.Fatal(err)
	}
	path := store.transcriptPath(id)
	if err := store.Clear(id); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("file should be removed: %v", err)
	}
}

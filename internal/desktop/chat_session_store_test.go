package desktop

import (
	"testing"

	"matrix/internal/matrixpaths"
)

func TestChatSessionStoreRoundTrip(t *testing.T) {
	matrixpaths.SetDataRootForTest(t.TempDir())
	t.Cleanup(func() { matrixpaths.SetDataRootForTest("") })

	ws := t.TempDir()
	store := NewChatSessionStore(func() string { return ws })
	sessions := []ChatSession{
		{
			ID:    "1",
			Title: "hello",
			Messages: []ChatMessage{
				{Role: "user", Content: "hi", Time: "12:00:00"},
			},
		},
	}
	if err := store.Save(sessions); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 1 || loaded[0].Title != "hello" {
		t.Fatalf("got %+v", loaded)
	}
}

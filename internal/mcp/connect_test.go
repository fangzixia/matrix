package mcp

import (
	"testing"
	"time"
)

func TestConnectDrawioMCP(t *testing.T) {
	if testing.Short() {
		t.Skip("short")
	}
	m := NewManager()
	m.UpdateConfigs(map[string]ServerConfig{
		"drawio-mcp": {
			Command: "npx",
			Args:    []string{"-y", "@drawio/mcp"},
		},
	})
	done := make(chan *ServerStatus, 1)
	go func() {
		done <- m.TestServer("drawio-mcp")
	}()
	select {
	case st := <-done:
		t.Logf("available=%v tools=%d err=%q", st.Available, st.ToolCount, st.Error)
		if !st.Available {
			t.Fatalf("expected available: %s", st.Error)
		}
	case <-time.After(120 * time.Second):
		t.Fatal("TestServer hung > 120s")
	}
}

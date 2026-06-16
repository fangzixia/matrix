package gitutil

import (
	"strings"
	"testing"
)

func TestDefaultSSHKeyPath(t *testing.T) {
	p := DefaultSSHKeyPath()
	if p == "" {
		t.Fatal("expected non-empty default path")
	}
	if !strings.Contains(p, ".ssh") {
		t.Fatalf("expected .ssh in path, got %q", p)
	}
	if !strings.HasSuffix(p, "id_rsa") {
		t.Fatalf("expected id_rsa suffix, got %q", p)
	}
}

func TestPlatformLabel(t *testing.T) {
	if PlatformLabel("windows") != "Windows" {
		t.Fatal("windows label")
	}
	if PlatformLabel("darwin") != "macOS" {
		t.Fatal("darwin label")
	}
	if PlatformLabel("linux") != "Linux" {
		t.Fatal("linux label")
	}
}

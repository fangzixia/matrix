package tools

import "testing"

func TestFormatHarnessUserMessage_sourceRunID(t *testing.T) {
	got := FormatHarnessUserMessage(`C:\repo`, `C:\docs`, "task", "11111111-1111-1111-1111-111111111111")
	want := "沙箱目录（源代码）: C:\\repo\n实现 Run（代码复制来源）: 11111111-1111-1111-1111-111111111111\n文档目录（计划/评测，非源码）: C:\\docs\n\ntask"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestFormatHarnessUserMessage_nilUUIDOmitted(t *testing.T) {
	got := FormatHarnessUserMessage(`C:\repo`, "", "task", "00000000-0000-0000-0000-000000000000")
	if got != "沙箱目录（源代码）: C:\\repo\n\ntask" {
		t.Fatalf("unexpected output: %q", got)
	}
}

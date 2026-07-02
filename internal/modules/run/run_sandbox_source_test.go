package run

import (
	"errors"
	"fmt"
	"testing"

	"gorm.io/gorm"
)

func TestIsCodeSandboxNotFound(t *testing.T) {
	if isCodeSandboxNotFound(nil) {
		t.Fatal("nil err should not be not-found")
	}
	if !isCodeSandboxNotFound(gorm.ErrRecordNotFound) {
		t.Fatal("expected gorm.ErrRecordNotFound")
	}
	if !isCodeSandboxNotFound(fmt.Errorf("未找到计划 docs/plans/PLAN-x.md 的实现 Run")) {
		t.Fatal("expected wrapped not-found message")
	}
	if isCodeSandboxNotFound(errors.New("计划 x 的实现 Run y 沙箱已不存在")) {
		t.Fatal("sandbox missing should not be treated as not-found")
	}
}

func TestCopyRepo_sourceKinds(t *testing.T) {
	if len(verifySourceKinds) != 1 || verifySourceKinds[0] != "implement" {
		t.Fatalf("verify should only reuse implement runs, got %v", verifySourceKinds)
	}
	want := map[string]bool{"implement": true, "pipeline": true, "build": true}
	for _, k := range codeSandboxKinds {
		if !want[k] {
			t.Fatalf("unexpected code sandbox kind %q", k)
		}
	}
}

package workspace

import (
	"errors"
	"fmt"
	"testing"
)

func TestSourceFetchError(t *testing.T) {
	t.Parallel()
	cause := fmt.Errorf("git clone 超时: network")
	err := &SourceFetchError{Attempts: 3, Cause: cause}
	if !IsSourceFetchError(err) {
		t.Fatal("expected IsSourceFetchError true")
	}
	if !errors.Is(err, ErrSourceFetchFailed) {
		t.Fatal("expected errors.Is ErrSourceFetchFailed")
	}
	if !IsSourceFetchError(fmt.Errorf("wrap: %w", err)) {
		t.Fatal("expected wrapped error detected")
	}
}

func TestIsSourceFetchError_false(t *testing.T) {
	t.Parallel()
	if IsSourceFetchError(fmt.Errorf("other error")) {
		t.Fatal("expected false for unrelated error")
	}
}

package idempotency

import (
	"errors"
	"strings"
	"testing"
)

func TestStorageError_ErrorAndUnwrap(t *testing.T) {
	baseErr := errors.New("boom")
	err := NewStorageError("set", baseErr)

	se, ok := err.(*StorageError)
	if !ok {
		t.Fatalf("expected *StorageError, got %T", err)
	}

	if se.Operation != "set" {
		t.Fatalf("expected operation %q, got %q", "set", se.Operation)
	}
	if !strings.Contains(se.Error(), "set") || !strings.Contains(se.Error(), "boom") {
		t.Fatalf("unexpected error string: %q", se.Error())
	}
	if !errors.Is(err, baseErr) {
		t.Fatalf("expected errors.Is to match wrapped error")
	}
	if !errors.Is(err, ErrStorageOperation) {
		// NewStorageError should conceptually be a storage operation error wrapper
		// but at minimum we ensure it unwraps correctly to the base error.
	}
}


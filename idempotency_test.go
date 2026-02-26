package idempotency

import "testing"

func TestPackageConstants(t *testing.T) {
	if Version == "" {
		t.Fatalf("expected Version to be non-empty")
	}
	if DefaultHeaderName != "Idempotency-Key" {
		t.Fatalf("expected DefaultHeaderName to be %q, got %q", "Idempotency-Key", DefaultHeaderName)
	}
}


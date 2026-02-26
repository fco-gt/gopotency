package key

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"

	idempotency "github.com/fco-gt/gopotency"
)

func TestHeaderBased_NoHeadersOrEmpty(t *testing.T) {
	strategy := HeaderBased("Idempotency-Key")

	tests := []struct {
		name    string
		headers map[string][]string
		want    string
	}{
		{
			name:    "nil headers",
			headers: nil,
			want:    "",
		},
		{
			name:    "missing header",
			headers: map[string][]string{"Other": {"value"}},
			want:    "",
		},
		{
			name:    "empty slice",
			headers: map[string][]string{"Idempotency-Key": {}},
			want:    "",
		},
		{
			name:    "with value",
			headers: map[string][]string{"Idempotency-Key": {"key-123", "ignored"}},
			want:    "key-123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &idempotency.Request{Headers: tt.headers}
			got, err := strategy.Generate(req)
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestBodyHash_GeneratesDeterministicKey(t *testing.T) {
	strategy := BodyHash()
	req := &idempotency.Request{
		Method: "POST",
		Path:   "/resource",
		Body:   []byte(`{"field":"value"}`),
	}

	got1, err := strategy.Generate(req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	got2, err := strategy.Generate(req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if got1 != got2 {
		t.Fatalf("expected deterministic key, got %q and %q", got1, got2)
	}

	expectedData := fmt.Sprintf("%s:%s:%s", req.Method, req.Path, string(req.Body))
	sum := sha256.Sum256([]byte(expectedData))
	expected := hex.EncodeToString(sum[:])
	if got1 != expected {
		t.Fatalf("expected %q, got %q", expected, got1)
	}
}

func TestComposite_PrefersHeaderAndFallsBackToBody(t *testing.T) {
	strategy := Composite("Idempotency-Key")

	t.Run("uses header when present", func(t *testing.T) {
		req := &idempotency.Request{
			Method: "POST",
			Path:   "/test",
			Body:   []byte("body"),
			Headers: map[string][]string{
				"Idempotency-Key": {"header-key"},
			},
		}

		got, err := strategy.Generate(req)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if got != "header-key" {
			t.Fatalf("expected header key, got %q", got)
		}
	})

	t.Run("falls back to body hash when header missing", func(t *testing.T) {
		req := &idempotency.Request{
			Method: "POST",
			Path:   "/test",
			Body:   []byte("body"),
		}

		got, err := strategy.Generate(req)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		expectedData := fmt.Sprintf("%s:%s:%s", req.Method, req.Path, string(req.Body))
		sum := sha256.Sum256([]byte(expectedData))
		expected := hex.EncodeToString(sum[:])

		if got != expected {
			t.Fatalf("expected %q, got %q", expected, got)
		}
	})
}


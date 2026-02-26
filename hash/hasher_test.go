package hash

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	idempotency "github.com/fco-gt/gopotency"
)

func TestBodyHasher_EmptyBody(t *testing.T) {
	hasher := BodyHasher()

	hash, err := hasher.Hash(&idempotency.Request{Body: nil})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if hash != "" {
		t.Fatalf("expected empty hash for empty body, got %q", hash)
	}
}

func TestBodyHasher_NonEmptyBody(t *testing.T) {
	hasher := BodyHasher()
	body := []byte("hello world")

	hash, err := hasher.Hash(&idempotency.Request{Body: body})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	sum := sha256.Sum256(body)
	expected := hex.EncodeToString(sum[:])
	if hash != expected {
		t.Fatalf("expected %q, got %q", expected, hash)
	}
}

func TestFullHasher_DeterministicAndSensitive(t *testing.T) {
	hasher := FullHasher()

	req := &idempotency.Request{
		Method: "POST",
		Path:   "/test",
		Body:   []byte("payload"),
	}

	hash1, err := hasher.Hash(req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	hash2, err := hasher.Hash(req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if hash1 != hash2 {
		t.Fatalf("expected deterministic hash, got %q and %q", hash1, hash2)
	}

	// Changing the body should change the hash
	req.Body = []byte("different payload")
	hash3, err := hasher.Hash(req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if hash3 == hash1 {
		t.Fatalf("expected different hash when body changes")
	}
}


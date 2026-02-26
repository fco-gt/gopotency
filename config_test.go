package idempotency

import (
	"context"
	"testing"
	"time"
)

type dummyStorage struct{}

func (d *dummyStorage) Get(ctx context.Context, key string) (*Record, error) { return nil, nil }
func (d *dummyStorage) Set(ctx context.Context, r *Record, ttl time.Duration) error {
	return nil
}
func (d *dummyStorage) Delete(ctx context.Context, key string) error         { return nil }
func (d *dummyStorage) Exists(ctx context.Context, key string) (bool, error) { return false, nil }
func (d *dummyStorage) TryLock(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	return true, nil
}
func (d *dummyStorage) Unlock(ctx context.Context, key string) error { return nil }
func (d *dummyStorage) Close() error                                 { return nil }

func TestConfig_SetDefaults(t *testing.T) {
	cfg := Config{
		Storage: &dummyStorage{},
	}

	cfg.setDefaults()

	if cfg.TTL == 0 {
		t.Errorf("expected TTL to be set, got 0")
	}
	if cfg.LockTimeout == 0 {
		t.Errorf("expected LockTimeout to be set, got 0")
	}
	if cfg.AllowedMethods == nil || len(cfg.AllowedMethods) == 0 {
		t.Errorf("expected default AllowedMethods to be set")
	}
	if cfg.RequestHasher == nil {
		t.Errorf("expected default RequestHasher to be set")
	}
}

func TestConfig_Validate(t *testing.T) {
	t.Run("missing storage", func(t *testing.T) {
		cfg := Config{}
		if err := cfg.validate(); err == nil {
			t.Fatalf("expected error for missing storage, got nil")
		}
	})

	t.Run("valid storage", func(t *testing.T) {
		cfg := Config{Storage: &dummyStorage{}}
		if err := cfg.validate(); err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
	})
}

func TestDefaultRequestHasher(t *testing.T) {
	hasher := &defaultRequestHasher{}

	t.Run("empty body returns empty hash", func(t *testing.T) {
		h, err := hasher.Hash(&Request{Body: nil})
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if h != "" {
			t.Fatalf("expected empty hash, got %q", h)
		}
	})

	t.Run("non-empty body returns hash", func(t *testing.T) {
		h, err := hasher.Hash(&Request{Body: []byte("data")})
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if h == "" {
			t.Fatalf("expected non-empty hash")
		}
	})
}


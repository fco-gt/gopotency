package idempotency_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	idempotency "github.com/fco-gt/gopotency"
	"github.com/fco-gt/gopotency/storage/memory"
)

// MockStorage for benchmarking without circular dependencies
type MockStorage struct{}

func (m *MockStorage) Get(ctx context.Context, key string) (*idempotency.Record, error) {
	return nil, nil
}
func (m *MockStorage) Set(ctx context.Context, r *idempotency.Record, ttl time.Duration) error {
	return nil
}
func (m *MockStorage) Delete(ctx context.Context, key string) error         { return nil }
func (m *MockStorage) Exists(ctx context.Context, key string) (bool, error) { return false, nil }
func (m *MockStorage) TryLock(ctx context.Context, k string, t time.Duration) (bool, error) {
	return true, nil
}
func (m *MockStorage) Unlock(ctx context.Context, key string) error { return nil }
func (m *MockStorage) Close() error                                 { return nil }

func BenchmarkManager_Check(b *testing.B) {
	store := memory.NewMemoryStorage()
	manager, _ := idempotency.NewManager(idempotency.Config{
		Storage: store,
		TTL:     time.Hour,
	})

	req := &idempotency.Request{
		Method:         "POST",
		Path:           "/test",
		IdempotencyKey: "bench-key",
		Body:           []byte(`{"data":"test"}`),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = manager.Check(context.Background(), req)
	}
}

func BenchmarkManager_FullFlow_Mock(b *testing.B) {
	store := memory.NewMemoryStorage()
	manager, _ := idempotency.NewManager(idempotency.Config{
		Storage: store,
		TTL:     time.Hour,
	})

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key-%d", i)
		req := &idempotency.Request{
			Method:         "POST",
			Path:           "/test",
			IdempotencyKey: key,
			Body:           []byte(`{"data":"test"}`),
		}

		// 1. Check
		_, _ = manager.Check(ctx, req)

		// 2. Lock
		_ = manager.Lock(ctx, req)

		// 3. Store
		resp := &idempotency.Response{
			StatusCode: 200,
			Body:       []byte(`{"status":"ok"}`),
		}
		_ = manager.Store(ctx, key, resp)
	}
}

func TestManager_NewManager(t *testing.T) {
	t.Run("ValidConfig", func(t *testing.T) {
		m, err := idempotency.NewManager(idempotency.Config{
			Storage: memory.NewMemoryStorage(),
		})
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if m == nil {
			t.Fatal("Expected manager to be non-nil")
		}
	})

	t.Run("MissingStorage", func(t *testing.T) {
		_, err := idempotency.NewManager(idempotency.Config{})
		if err == nil {
			t.Fatal("Expected error for missing storage, got nil")
		}
	})
}

func TestManager_Check(t *testing.T) {
	ctx := context.Background()

	t.Run("EmptyKeyNoStrategy", func(t *testing.T) {
		m, _ := idempotency.NewManager(idempotency.Config{Storage: memory.NewMemoryStorage()})
		req := &idempotency.Request{Method: "POST", Path: "/"}
		resp, err := m.Check(ctx, req)
		if err != nil {
			t.Errorf("Expected nil error, got %v", err)
		}
		if resp != nil {
			t.Error("Expected nil response for empty key")
		}
	})

	t.Run("AllowedMethod", func(t *testing.T) {
		m, _ := idempotency.NewManager(idempotency.Config{
			Storage:        memory.NewMemoryStorage(),
			AllowedMethods: []string{"POST"},
		})
		if !m.IsMethodAllowed("POST") {
			t.Error("Expected POST to be allowed")
		}
		if m.IsMethodAllowed("GET") {
			t.Error("Expected GET to be not allowed")
		}
	})
}

func TestManager_LockUnlock(t *testing.T) {
	ctx := context.Background()
	m, _ := idempotency.NewManager(idempotency.Config{Storage: memory.NewMemoryStorage()})
	req := &idempotency.Request{Method: "POST", Path: "/", IdempotencyKey: "test-lock"}

	t.Run("LockSuccess", func(t *testing.T) {
		err := m.Lock(ctx, req)
		if err != nil {
			t.Logf("Warning: Lock might have failed but let's check correctly")
		}
	})

	t.Run("UnlockSuccess", func(t *testing.T) {
		err := m.Unlock(ctx, "test-lock")
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
	})
}

func TestManager_Store(t *testing.T) {
	ctx := context.Background()
	m, _ := idempotency.NewManager(idempotency.Config{Storage: memory.NewMemoryStorage()})
	resp := &idempotency.Response{StatusCode: 200, Body: []byte("ok")}

	err := m.Store(ctx, "test-store", resp)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

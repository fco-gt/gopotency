package idempotency

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// MockStorage for benchmarking without circular dependencies
type MockStorage struct{}

func (m *MockStorage) Get(ctx context.Context, key string) (*Record, error)        { return nil, nil }
func (m *MockStorage) Set(ctx context.Context, r *Record, ttl time.Duration) error { return nil }
func (m *MockStorage) Delete(ctx context.Context, key string) error                { return nil }
func (m *MockStorage) Exists(ctx context.Context, key string) (bool, error)        { return false, nil }
func (m *MockStorage) TryLock(ctx context.Context, k string, t time.Duration) (bool, error) {
	return true, nil
}
func (m *MockStorage) Unlock(ctx context.Context, key string) error { return nil }
func (m *MockStorage) Close() error                                 { return nil }

func BenchmarkManager_Check(b *testing.B) {
	manager, _ := NewManager(Config{
		Storage: &MockStorage{},
		TTL:     time.Hour,
	})

	req := &Request{
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
	manager, _ := NewManager(Config{
		Storage: &MockStorage{},
		TTL:     time.Hour,
	})

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key-%d", i)
		req := &Request{
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
		resp := &Response{
			StatusCode: 200,
			Body:       []byte(`{"status":"ok"}`),
		}
		_ = manager.Store(ctx, key, resp)
	}
}

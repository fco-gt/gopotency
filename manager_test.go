package idempotency

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// MockStorage for benchmarking without circular dependencies
type MockStorage struct {
	GetFunc     func(ctx context.Context, key string) (*Record, error)
	SetFunc     func(ctx context.Context, r *Record, ttl time.Duration) error
	DeleteFunc  func(ctx context.Context, key string) error
	ExistsFunc  func(ctx context.Context, key string) (bool, error)
	TryLockFunc func(ctx context.Context, k string, t time.Duration) (bool, error)
	UnlockFunc  func(ctx context.Context, key string) error
	CloseFunc   func() error
}

func (m *MockStorage) Get(ctx context.Context, key string) (*Record, error) {
	if m.GetFunc != nil {
		return m.GetFunc(ctx, key)
	}
	return nil, nil
}
func (m *MockStorage) Set(ctx context.Context, r *Record, ttl time.Duration) error {
	if m.SetFunc != nil {
		return m.SetFunc(ctx, r, ttl)
	}
	return nil
}
func (m *MockStorage) Delete(ctx context.Context, key string) error {
	if m.DeleteFunc != nil {
		return m.DeleteFunc(ctx, key)
	}
	return nil
}
func (m *MockStorage) Exists(ctx context.Context, key string) (bool, error) {
	if m.ExistsFunc != nil {
		return m.ExistsFunc(ctx, key)
	}
	return false, nil
}
func (m *MockStorage) TryLock(ctx context.Context, k string, t time.Duration) (bool, error) {
	if m.TryLockFunc != nil {
		return m.TryLockFunc(ctx, k, t)
	}
	return true, nil
}
func (m *MockStorage) Unlock(ctx context.Context, key string) error {
	if m.UnlockFunc != nil {
		return m.UnlockFunc(ctx, key)
	}
	return nil
}
func (m *MockStorage) Close() error {
	if m.CloseFunc != nil {
		return m.CloseFunc()
	}
	return nil
}

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
func TestManager_IsMethodAllowed(t *testing.T) {
	t.Run("DefaultMethods", func(t *testing.T) {
		m, _ := NewManager(Config{Storage: &MockStorage{}})
		methods := []string{"POST", "PUT", "PATCH", "DELETE"}
		for _, method := range methods {
			if !m.IsMethodAllowed(method) {
				t.Errorf("Expected %s to be allowed", method)
			}
		}
		if m.IsMethodAllowed("GET") {
			t.Error("Expected GET to be forbidden by default")
		}
	})

	t.Run("CustomMethods", func(t *testing.T) {
		m, _ := NewManager(Config{
			Storage:        &MockStorage{},
			AllowedMethods: []string{"GET"},
		})
		if !m.IsMethodAllowed("GET") {
			t.Error("Expected GET to be allowed")
		}
		if m.IsMethodAllowed("POST") {
			t.Error("Expected POST to be forbidden")
		}
	})

	t.Run("EmptyMethods", func(t *testing.T) {
		m, _ := NewManager(Config{
			Storage:        &MockStorage{},
			AllowedMethods: []string{},
		})
		if m.IsMethodAllowed("POST") {
			t.Error("Expected no methods to be allowed")
		}
	})
}

func TestManager_Close(t *testing.T) {
	m, _ := NewManager(Config{Storage: &MockStorage{}})
	if err := m.Close(); err != nil {
		t.Errorf("Expected nil error, got %v", err)
	}
}

func TestManager_Check_EdgeCases(t *testing.T) {
	ctx := context.Background()

	t.Run("MissingKey_RequireKeyTrue", func(t *testing.T) {
		m, _ := NewManager(Config{
			Storage:    &MockStorage{},
			RequireKey: true,
		})
		req := &Request{Method: "POST", Path: "/"}
		_, err := m.Check(ctx, req)
		if err == nil {
			t.Error("Expected error for missing key with RequireKey=true")
		}
	})

	t.Run("RecordFound_Replay", func(t *testing.T) {
		expectedRecord := &Record{
			Key:    "test-key",
			Status: StatusCompleted,
			Response: &CachedResponse{
				StatusCode: 201,
				Body:       []byte("created"),
			},
		}
		m, _ := NewManager(Config{
			Storage: &MockStorage{
				GetFunc: func(ctx context.Context, key string) (*Record, error) {
					return expectedRecord, nil
				},
			},
		})
		req := &Request{Method: "POST", Path: "/", IdempotencyKey: "test-key"}
		record, err := m.Check(ctx, req)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if record == nil || record.StatusCode != 201 {
			t.Error("Expected record with status 201 to be returned")
		}
	})

	t.Run("RecordExpired_Delete", func(t *testing.T) {
		deleted := false
		m, _ := NewManager(Config{
			Storage: &MockStorage{
				GetFunc: func(ctx context.Context, key string) (*Record, error) {
					return &Record{
						Key:       "test-key",
						ExpiresAt: time.Now().Add(-time.Hour),
					}, nil
				},
				DeleteFunc: func(ctx context.Context, key string) error {
					deleted = true
					return nil
				},
			},
		})
		req := &Request{Method: "POST", Path: "/", IdempotencyKey: "test-key"}
		record, err := m.Check(ctx, req)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if record != nil {
			t.Error("Expected nil record for expired one")
		}
		if !deleted {
			t.Error("Expected expired record to be deleted")
		}
	})

	t.Run("RecordPending_Error", func(t *testing.T) {
		m, _ := NewManager(Config{
			Storage: &MockStorage{
				GetFunc: func(ctx context.Context, key string) (*Record, error) {
					return &Record{
						Key:    "test-key",
						Status: StatusPending,
					}, nil
				},
			},
		})
		req := &Request{Method: "POST", Path: "/", IdempotencyKey: "test-key"}
		_, err := m.Check(ctx, req)
		if err == nil {
			t.Error("Expected error for pending request")
		}
	})
}

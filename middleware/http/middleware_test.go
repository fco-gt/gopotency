package http

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	idempotency "github.com/fco-gt/gopotency"
)

// MockStorage for middleware testing
type MockStorage struct {
	Records map[string]*idempotency.Record
	Locks   map[string]bool
}

func (m *MockStorage) Get(ctx context.Context, key string) (*idempotency.Record, error) {
	return m.Records[key], nil
}
func (m *MockStorage) Set(ctx context.Context, r *idempotency.Record, ttl time.Duration) error {
	m.Records[r.Key] = r
	return nil
}
func (m *MockStorage) Delete(ctx context.Context, key string) error {
	delete(m.Records, key)
	return nil
}
func (m *MockStorage) Exists(ctx context.Context, key string) (bool, error) {
	_, ok := m.Records[key]
	return ok, nil
}
func (m *MockStorage) TryLock(ctx context.Context, k string, t time.Duration) (bool, error) {
	if m.Locks[k] {
		return false, nil
	}
	m.Locks[k] = true
	return true, nil
}
func (m *MockStorage) Unlock(ctx context.Context, key string) error { delete(m.Locks, key); return nil }
func (m *MockStorage) Close() error                                 { return nil }

func TestIdempotencyMiddleware(t *testing.T) {
	store := &MockStorage{
		Records: make(map[string]*idempotency.Record),
		Locks:   make(map[string]bool),
	}
	manager, _ := idempotency.NewManager(idempotency.Config{
		Storage: store,
	})

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	middleware := Idempotency(manager)(handler)

	t.Run("FirstRequest_Success", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/test", bytes.NewBuffer([]byte("data")))
		req.Header.Set("Idempotency-Key", "key1")
		w := httptest.NewRecorder()

		middleware.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected 200, got %d", w.Code)
		}
		if w.Body.String() != "ok" {
			t.Errorf("Expected 'ok', got '%s'", w.Body.String())
		}
	})

	t.Run("SecondRequest_Replay", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/test", bytes.NewBuffer([]byte("data")))
		req.Header.Set("Idempotency-Key", "key1")
		w := httptest.NewRecorder()

		middleware.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected 200, got %d", w.Code)
		}
		if w.Header().Get("X-Idempotent-Replayed") != "true" {
			t.Error("Expected X-Idempotent-Replayed header to be true")
		}
	})

	t.Run("Conflict_InProgress", func(t *testing.T) {
		store.Locks["key-locking"] = true
		req := httptest.NewRequest("POST", "/test", bytes.NewBuffer([]byte("data")))
		req.Header.Set("Idempotency-Key", "key-locking")
		w := httptest.NewRecorder()

		middleware.ServeHTTP(w, req)

		if w.Code != http.StatusConflict {
			t.Errorf("Expected 409 Conflict, got %d", w.Code)
		}
	})

	t.Run("RequireKey_Failure", func(t *testing.T) {
		m2, _ := idempotency.NewManager(idempotency.Config{
			Storage:    store,
			RequireKey: true,
		})
		mw2 := Idempotency(m2)(handler)

		req := httptest.NewRequest("POST", "/test", nil)
		w := httptest.NewRecorder()

		mw2.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected 400 Bad Request, got %d", w.Code)
		}
	})
}

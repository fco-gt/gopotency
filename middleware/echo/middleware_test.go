package echo

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	idempotency "github.com/fco-gt/gopotency"
	"github.com/labstack/echo/v4"
)

// MockStorage for Echo middleware testing
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

func TestEchoIdempotency(t *testing.T) {
	e := echo.New()
	store := &MockStorage{
		Records: make(map[string]*idempotency.Record),
		Locks:   make(map[string]bool),
	}
	manager, _ := idempotency.NewManager(idempotency.Config{
		Storage: store,
	})

	e.Use(Idempotency(manager))

	e.POST("/test", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	t.Run("FirstRequest", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/test", bytes.NewBuffer([]byte("data")))
		req.Header.Set("Idempotency-Key", "echo-key")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
	})

	t.Run("ReplayRequest", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/test", bytes.NewBuffer([]byte("data")))
		req.Header.Set("Idempotency-Key", "echo-key")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		if rec.Header().Get("X-Idempotent-Replayed") != "true" {
			t.Error("expected replay header")
		}
	})

	t.Run("Conflict_InProgress", func(t *testing.T) {
		store.Locks["echo-conflict"] = true
		req := httptest.NewRequest("POST", "/test", bytes.NewBuffer([]byte("data")))
		req.Header.Set("Idempotency-Key", "echo-conflict")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		if rec.Code != http.StatusConflict {
			t.Errorf("expected 409, got %d", rec.Code)
		}
	})

	t.Run("RequireKey_Failure", func(t *testing.T) {
		m2, _ := idempotency.NewManager(idempotency.Config{
			Storage:    store,
			RequireKey: true,
		})
		e2 := echo.New()
		e2.Use(Idempotency(m2))
		e2.POST("/test", func(c echo.Context) error {
			return c.String(http.StatusOK, "ok")
		})

		req := httptest.NewRequest("POST", "/test", nil)
		rec := httptest.NewRecorder()
		e2.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", rec.Code)
		}
	})
}

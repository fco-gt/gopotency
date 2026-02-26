package fiber

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	idempotency "github.com/fco-gt/gopotency"
	"github.com/gofiber/fiber/v2"
)

// MockStorage for Fiber middleware testing
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

func TestFiberIdempotency(t *testing.T) {
	app := fiber.New()
	store := &MockStorage{
		Records: make(map[string]*idempotency.Record),
		Locks:   make(map[string]bool),
	}
	manager, _ := idempotency.NewManager(idempotency.Config{
		Storage: store,
	})

	app.Use(Idempotency(manager))

	app.Post("/test", func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})

	t.Run("FirstRequest", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/test", bytes.NewBuffer([]byte("data")))
		req.Header.Set("Idempotency-Key", "fiber-key")
		resp, _ := app.Test(req)

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("ReplayRequest", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/test", bytes.NewBuffer([]byte("data")))
		req.Header.Set("Idempotency-Key", "fiber-key")
		resp, _ := app.Test(req)

		if resp.Header.Get("X-Idempotent-Replayed") != "true" {
			t.Error("expected replay header")
		}
	})
}

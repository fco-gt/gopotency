package gin_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	idempotency "github.com/fco-gt/gopotency"
	ginmw "github.com/fco-gt/gopotency/middleware/gin"
	"github.com/gin-gonic/gin"
)

// MockStorage for Gin middleware testing
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

func TestGinIdempotency(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &MockStorage{
		Records: make(map[string]*idempotency.Record),
		Locks:   make(map[string]bool),
	}
	manager, _ := idempotency.NewManager(idempotency.Config{
		Storage: store,
	})

	r := gin.New()
	r.Use(ginmw.Idempotency(manager))

	count := 0
	r.POST("/test", func(c *gin.Context) {
		count++
		c.JSON(200, gin.H{"count": count})
	})

	t.Run("FirstRequest", func(t *testing.T) {
		req, _ := http.NewRequest("POST", "/test", bytes.NewBuffer([]byte("data")))
		req.Header.Set("Idempotency-Key", "gin-key")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
	})

	t.Run("ReplayRequest", func(t *testing.T) {
		req, _ := http.NewRequest("POST", "/test", bytes.NewBuffer([]byte("data")))
		req.Header.Set("Idempotency-Key", "gin-key")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Header().Get("X-Idempotent-Replayed") != "true" {
			t.Error("expected replay header")
		}

		var resp map[string]int
		json.Unmarshal(w.Body.Bytes(), &resp)
		if resp["count"] != 1 {
			t.Errorf("expected count 1, got %d", resp["count"])
		}
	})

	t.Run("Conflict_InProgress", func(t *testing.T) {
		store.Locks["gin-conflict"] = true
		req, _ := http.NewRequest("POST", "/test", bytes.NewBuffer([]byte("data")))
		req.Header.Set("Idempotency-Key", "gin-conflict")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusConflict {
			t.Errorf("expected 409, got %d", w.Code)
		}
	})

	t.Run("RequireKey_Failure", func(t *testing.T) {
		m2, _ := idempotency.NewManager(idempotency.Config{
			Storage:    store,
			RequireKey: true,
		})
		r2 := gin.New()
		r2.Use(ginmw.Idempotency(m2))
		r2.POST("/test", func(c *gin.Context) {
			c.Status(200)
		})

		req, _ := http.NewRequest("POST", "/test", nil)
		w := httptest.NewRecorder()
		r2.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", w.Code)
		}
	})

	t.Run("WriteString", func(t *testing.T) {
		r3 := gin.New()
		r3.Use(ginmw.Idempotency(manager))
		r3.POST("/test", func(c *gin.Context) {
			_, _ = c.Writer.WriteString("hello string")
		})

		req, _ := http.NewRequest("POST", "/test", nil)
		req.Header.Set("Idempotency-Key", "gin-string-key")
		w := httptest.NewRecorder()
		r3.ServeHTTP(w, req)

		if w.Body.String() != "hello string" {
			t.Errorf("expected body 'hello string', got '%s'", w.Body.String())
		}
	})
}

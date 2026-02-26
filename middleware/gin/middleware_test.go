package gin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	idempotency "github.com/fco-gt/gopotency"
	"github.com/fco-gt/gopotency/storage/memory"
	"github.com/gin-gonic/gin"
)

// localMockStorage mirrors the simple in-memory storage used in other middleware tests
type localMockStorage struct {
	Records map[string]*idempotency.Record
	Locks   map[string]bool
}

func (m *localMockStorage) Get(ctx context.Context, key string) (*idempotency.Record, error) {
	return m.Records[key], nil
}

func (m *localMockStorage) Set(ctx context.Context, r *idempotency.Record, ttl time.Duration) error {
	m.Records[r.Key] = r
	return nil
}

func (m *localMockStorage) Delete(ctx context.Context, key string) error {
	delete(m.Records, key)
	return nil
}

func (m *localMockStorage) Exists(ctx context.Context, key string) (bool, error) {
	_, ok := m.Records[key]
	return ok, nil
}

func (m *localMockStorage) TryLock(ctx context.Context, k string, t time.Duration) (bool, error) {
	if m.Locks[k] {
		return false, nil
	}
	m.Locks[k] = true
	return true, nil
}

func (m *localMockStorage) Unlock(ctx context.Context, key string) error {
	delete(m.Locks, key)
	return nil
}

func (m *localMockStorage) Close() error { return nil }

func TestGinIdempotency_BasicReplay(t *testing.T) {
	gin.SetMode(gin.TestMode)

	store := &localMockStorage{
		Records: make(map[string]*idempotency.Record),
		Locks:   make(map[string]bool),
	}
	manager, _ := idempotency.NewManager(idempotency.Config{
		Storage: store,
	})

	r := gin.New()
	r.Use(Idempotency(manager))

	counter := 0
	r.POST("/test", func(c *gin.Context) {
		counter++
		c.JSON(http.StatusOK, gin.H{"count": counter})
	})

	body := []byte(`{"foo":"bar"}`)

	// First request
	req1, _ := http.NewRequest("POST", "/test", bytes.NewReader(body))
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("Idempotency-Key", "gin-key")
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)

	if w1.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w1.Code)
	}

	// Second (replayed) request
	req2, _ := http.NewRequest("POST", "/test", bytes.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Idempotency-Key", "gin-key")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w2.Code)
	}
	if w2.Header().Get("X-Idempotent-Replayed") != "true" {
		t.Fatalf("expected X-Idempotent-Replayed header to be true")
	}

	var resp map[string]int
	if err := json.Unmarshal(w2.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp["count"] != 1 {
		t.Fatalf("expected handler to run once, count=1, got %d", resp["count"])
	}
}

func TestGinIdempotency_RequireKeyAndConflict(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("RequireKey should reject missing key", func(t *testing.T) {
		store := memory.NewMemoryStorage()
		manager, _ := idempotency.NewManager(idempotency.Config{
			Storage:        store,
			AllowedMethods: []string{"POST"},
			RequireKey:     true,
		})

		r := gin.New()
		r.Use(Idempotency(manager))
		r.POST("/critical", func(c *gin.Context) {
			c.Status(http.StatusOK)
		})

		req, _ := http.NewRequest("POST", "/critical", bytes.NewReader([]byte(`{}`)))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", w.Code)
		}
	})

	t.Run("Lock conflict returns 409", func(t *testing.T) {
		store := &localMockStorage{
			Records: make(map[string]*idempotency.Record),
			Locks:   make(map[string]bool),
		}
		manager, _ := idempotency.NewManager(idempotency.Config{
			Storage: store,
		})

		r := gin.New()
		r.Use(Idempotency(manager))
		r.POST("/lock", func(c *gin.Context) {
			c.Status(http.StatusOK)
		})

		// Pre-mark the lock as held so TryLock fails
		store.Locks["conflict-key"] = true

		req, _ := http.NewRequest("POST", "/lock", bytes.NewReader([]byte(`payload`)))
		req.Header.Set("Idempotency-Key", "conflict-key")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusConflict {
			t.Fatalf("expected 409 Conflict, got %d", w.Code)
		}
	})
}


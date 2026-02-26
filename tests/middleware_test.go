package idempotency_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	idempotency "github.com/fco-gt/gopotency"
	ginmw "github.com/fco-gt/gopotency/middleware/gin"
	"github.com/fco-gt/gopotency/storage/memory"
	"github.com/gin-gonic/gin"
)

func TestMiddleware_RouteSpecific(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("GET with key should be idempotent even if not in AllowedMethods", func(t *testing.T) {
		store := memory.NewMemoryStorage()
		manager, _ := idempotency.NewManager(idempotency.Config{
			Storage:        store,
			AllowedMethods: []string{"POST"}, // GET not allowed globally
		})

		r := gin.New()
		r.Use(ginmw.Idempotency(manager))

		count := 0
		r.GET("/test", func(c *gin.Context) {
			count++
			c.JSON(200, gin.H{"count": count})
		})

		// First request
		req1, _ := http.NewRequest("GET", "/test", nil)
		req1.Header.Set("Idempotency-Key", "test-key-get")
		w1 := httptest.NewRecorder()
		r.ServeHTTP(w1, req1)

		if w1.Code != 200 {
			t.Errorf("expected 200, got %d", w1.Code)
		}

		// Second request (replayed)
		req2, _ := http.NewRequest("GET", "/test", nil)
		req2.Header.Set("Idempotency-Key", "test-key-get")
		w2 := httptest.NewRecorder()
		r.ServeHTTP(w2, req2)

		if w2.Code != 200 {
			t.Errorf("expected 200, got %d", w2.Code)
		}
		if w2.Header().Get("X-Idempotent-Replayed") != "true" {
			t.Error("expected X-Idempotent-Replayed header")
		}

		var resp map[string]int
		json.Unmarshal(w2.Body.Bytes(), &resp)
		if resp["count"] != 1 {
			t.Errorf("expected count 1, got %d", resp["count"])
		}
	})

	t.Run("RequireKey SHOULD error if key is missing for allowed method", func(t *testing.T) {
		store := memory.NewMemoryStorage()
		manager, _ := idempotency.NewManager(idempotency.Config{
			Storage:        store,
			AllowedMethods: []string{"POST"},
			RequireKey:     true,
		})

		r := gin.New()
		r.Use(ginmw.Idempotency(manager))
		r.POST("/critical", func(c *gin.Context) {
			c.Status(200)
		})

		req, _ := http.NewRequest("POST", "/critical", bytes.NewBuffer([]byte(`{}`)))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", w.Code)
		}
	})
}

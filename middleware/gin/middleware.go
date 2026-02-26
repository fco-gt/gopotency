// Package gin provides Gin Gonic middleware for idempotency handling.
package gin

import (
	"bytes"
	"io"
	"net/http"

	idempotency "github.com/fco-gt/gopotency"
	"github.com/gin-gonic/gin"
)

// Idempotency returns a Gin middleware that handles idempotency
func Idempotency(manager *idempotency.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. Extract potential idempotency key from header
		headerKey := c.GetHeader("Idempotency-Key")

		// 2. Build dummy request for potential auto-generation
		// We avoid reading the body until we are sure we need it
		pReq := &idempotency.Request{
			Method:         c.Request.Method,
			Path:           c.Request.URL.Path,
			Headers:        c.Request.Header,
			IdempotencyKey: headerKey,
		}

		// 3. Determine if we should apply idempotency
		isMethodAllowed := manager.IsMethodAllowed(c.Request.Method)
		hasKey := headerKey != ""

		// If no header key, check if we should generate one or if it's even allowed
		if !hasKey {
			// If method is not allowed AND we don't have a header key, skip early
			if !isMethodAllowed {
				c.Next()
				return
			}

			// If method is allowed, check if we have a strategy to generate it
			// This might require reading the body later if it's a BodyHash strategy
		}

		// 4. Handle Request Body (if needed for idempotency or just to be safe)
		var body []byte
		if c.Request.Body != nil {
			var err error
			body, err = io.ReadAll(c.Request.Body)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read request body"})
				c.Abort()
				return
			}
			c.Request.Body = io.NopCloser(bytes.NewBuffer(body))
		}
		pReq.Body = body

		// 5. Final Key Generation (e.g. if using BodyHash and header was empty)
		if pReq.IdempotencyKey == "" {
			// Get manager.config to access the strategy
			// (Assuming we might need to expose Strategy better or just call it)
			// But for now, we follow the manager.Check logic which calls it.
		}

		// 6. Check for cached response
		cachedResp, err := manager.Check(c.Request.Context(), pReq)
		if err != nil {
			if err == idempotency.ErrRequestInProgress {
				c.JSON(http.StatusConflict, gin.H{"error": "request already in progress"})
				c.Abort()
				return
			}
			if err == idempotency.ErrRequestMismatch {
				c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "idempotency key reused with different payload"})
				c.Abort()
				return
			}
			// Other errors proceed normally
		}

		// 7. Missing Key Handling (RequireKey check)
		// At this point, Check() should have populated IdempotencyKey if it could.
		if pReq.IdempotencyKey == "" {
			// If it's a method that usually requires it (or global list) and RequireKey is on
			if manager.Config().RequireKey && isMethodAllowed {
				c.JSON(http.StatusBadRequest, gin.H{"error": "idempotency key is required for this request"})
				c.Abort()
				return
			}
			// Otherwise just skip
			c.Next()
			return
		}

		// 8. Return cached response if available
		if cachedResp != nil {
			for key, values := range cachedResp.Headers {
				for _, value := range values {
					c.Header(key, value)
				}
			}
			c.Header("X-Idempotent-Replayed", "true")
			c.Data(cachedResp.StatusCode, cachedResp.ContentType, cachedResp.Body)
			c.Abort()
			return
		}

		// 9. Acquire lock
		if err := manager.Lock(c.Request.Context(), pReq); err != nil {
			if err == idempotency.ErrRequestInProgress {
				c.JSON(http.StatusConflict, gin.H{"error": "request already in progress"})
				c.Abort()
				return
			}
			// Proceed without idempotency if lock fails for other reasons?
			// Usually safer to allow but depends on preference.
			// We follow existing logic.
		}

		// 10. Capture response
		writer := &responseWriter{
			ResponseWriter: c.Writer,
			body:           &bytes.Buffer{},
		}
		c.Writer = writer

		c.Next()

		// 11. Store response
		if pReq.IdempotencyKey != "" && c.Writer.Status() < 500 {
			resp := &idempotency.Response{
				StatusCode:  c.Writer.Status(),
				Headers:     c.Writer.Header().Clone(),
				Body:        writer.body.Bytes(),
				ContentType: c.Writer.Header().Get("Content-Type"),
			}
			_ = manager.Store(c.Request.Context(), pReq.IdempotencyKey, resp)
		} else if pReq.IdempotencyKey != "" {
			_ = manager.Unlock(c.Request.Context(), pReq.IdempotencyKey)
		}
	}
}

// responseWriter wraps gin.ResponseWriter to capture response body
type responseWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w *responseWriter) Write(data []byte) (int, error) {
	w.body.Write(data)
	return w.ResponseWriter.Write(data)
}

func (w *responseWriter) WriteString(s string) (int, error) {
	w.body.WriteString(s)
	return w.ResponseWriter.WriteString(s)
}

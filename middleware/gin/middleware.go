// Package gin provides Gin Gonic middleware for idempotency handling.
package gin

import (
	"bytes"
	"io"
	"net/http"

	"github.com/fco-gt/idempotency-go"
	"github.com/gin-gonic/gin"
)

// Idempotency returns a Gin middleware that handles idempotency
func Idempotency(manager *idempotency.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip if method is not allowed
		if !manager.IsMethodAllowed(c.Request.Method) {
			c.Next()
			return
		}

		// Read body
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read request body"})
			c.Abort()
			return
		}

		// Restore body for downstream handlers
		c.Request.Body = io.NopCloser(bytes.NewBuffer(body))

		// Build request object
		req := &idempotency.Request{
			Method:  c.Request.Method,
			Path:    c.Request.URL.Path,
			Headers: c.Request.Header,
			Body:    body,
		}

		// Extract idempotency key from header if present
		if key := c.GetHeader("Idempotency-Key"); key != "" {
			req.IdempotencyKey = key
		}

		// Check for cached response
		cachedResp, err := manager.Check(c.Request.Context(), req)
		if err != nil {
			if err == idempotency.ErrRequestInProgress {
				c.JSON(http.StatusConflict, gin.H{
					"error": "request with this idempotency key is already being processed",
				})
				c.Abort()
				return
			}

			if err == idempotency.ErrRequestMismatch {
				c.JSON(http.StatusUnprocessableEntity, gin.H{
					"error": "request with same idempotency key has different content",
				})
				c.Abort()
				return
			}

			// Other errors, proceed without idempotency
			c.Next()
			return
		}

		// Return cached response if available
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

		// Acquire lock if we have an idempotency key
		if req.IdempotencyKey != "" {
			if err := manager.Lock(c.Request.Context(), req); err != nil {
				if err == idempotency.ErrRequestInProgress {
					c.JSON(http.StatusConflict, gin.H{
						"error": "request with this idempotency key is already being processed",
					})
					c.Abort()
					return
				}
				// Other errors, proceed without idempotency
			}
		}

		// Capture response
		writer := &responseWriter{
			ResponseWriter: c.Writer,
			body:           &bytes.Buffer{},
		}
		c.Writer = writer

		// Process request
		c.Next()

		// Store response if we have an idempotency key and processing succeeded
		if req.IdempotencyKey != "" && c.Writer.Status() < 500 {
			resp := &idempotency.Response{
				StatusCode:  c.Writer.Status(),
				Headers:     c.Writer.Header().Clone(),
				Body:        writer.body.Bytes(),
				ContentType: c.Writer.Header().Get("Content-Type"),
			}

			_ = manager.Store(c.Request.Context(), req.IdempotencyKey, resp)
		} else if req.IdempotencyKey != "" {
			// Unlock on error
			_ = manager.Unlock(c.Request.Context(), req.IdempotencyKey)
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

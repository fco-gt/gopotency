// Package http provides standard library HTTP middleware for idempotency handling.
package http

import (
	"bytes"
	"io"
	"net/http"

	"github.com/fco-gt/idempotency-go"
)

// Idempotency returns an HTTP middleware that handles idempotency
func Idempotency(manager *idempotency.Manager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip if method is not allowed
			if !manager.IsMethodAllowed(r.Method) {
				next.ServeHTTP(w, r)
				return
			}

			// Read body
			body, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, "failed to read request body", http.StatusBadRequest)
				return
			}
			r.Body.Close()

			// Restore body for downstream handlers
			r.Body = io.NopCloser(bytes.NewBuffer(body))

			// Build request object
			req := &idempotency.Request{
				Method:  r.Method,
				Path:    r.URL.Path,
				Headers: r.Header,
				Body:    body,
			}

			// Extract idempotency key from header if present
			if key := r.Header.Get("Idempotency-Key"); key != "" {
				req.IdempotencyKey = key
			}

			// Check for cached response
			cachedResp, err := manager.Check(r.Context(), req)
			if err != nil {
				if err == idempotency.ErrRequestInProgress {
					http.Error(w, `{"error":"request with this idempotency key is already being processed"}`, http.StatusConflict)
					return
				}

				if err == idempotency.ErrRequestMismatch {
					http.Error(w, `{"error":"request with same idempotency key has different content"}`, http.StatusUnprocessableEntity)
					return
				}

				// Other errors, proceed without idempotency
				next.ServeHTTP(w, r)
				return
			}

			// Return cached response if available
			if cachedResp != nil {
				for key, values := range cachedResp.Headers {
					for _, value := range values {
						w.Header().Add(key, value)
					}
				}
				w.Header().Set("X-Idempotent-Replayed", "true")
				w.WriteHeader(cachedResp.StatusCode)
				w.Write(cachedResp.Body)
				return
			}

			// Acquire lock if we have an idempotency key
			if req.IdempotencyKey != "" {
				if err := manager.Lock(r.Context(), req); err != nil {
					if err == idempotency.ErrRequestInProgress {
						http.Error(w, `{"error":"request with this idempotency key is already being processed"}`, http.StatusConflict)
						return
					}
					// Other errors, proceed without idempotency
				}
			}

			// Capture response
			recorder := &responseRecorder{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
				body:           &bytes.Buffer{},
			}

			// Process request
			next.ServeHTTP(recorder, r)

			// Store response if we have an idempotency key and processing succeeded
			if req.IdempotencyKey != "" && recorder.statusCode < 500 {
				resp := &idempotency.Response{
					StatusCode:  recorder.statusCode,
					Headers:     recorder.Header().Clone(),
					Body:        recorder.body.Bytes(),
					ContentType: recorder.Header().Get("Content-Type"),
				}

				_ = manager.Store(r.Context(), req.IdempotencyKey, resp)
			} else if req.IdempotencyKey != "" {
				// Unlock on error
				_ = manager.Unlock(r.Context(), req.IdempotencyKey)
			}
		})
	}
}

// responseRecorder wraps http.ResponseWriter to capture response
type responseRecorder struct {
	http.ResponseWriter
	statusCode int
	body       *bytes.Buffer
}

func (r *responseRecorder) WriteHeader(statusCode int) {
	r.statusCode = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

func (r *responseRecorder) Write(data []byte) (int, error) {
	r.body.Write(data)
	return r.ResponseWriter.Write(data)
}

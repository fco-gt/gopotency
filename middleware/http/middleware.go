// Package http provides standard library HTTP middleware for idempotency handling.
package http

import (
	"bytes"
	"io"
	"net/http"

	idempotency "github.com/fco-gt/gopotency"
)

// Idempotency returns an HTTP middleware that handles idempotency
func Idempotency(manager *idempotency.Manager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 1. Extract potential idempotency key from header
			headerKey := r.Header.Get("Idempotency-Key")

			// 2. Build dummy request for potential auto-generation
			pReq := &idempotency.Request{
				Method:         r.Method,
				Path:           r.URL.Path,
				Headers:        r.Header,
				IdempotencyKey: headerKey,
			}

			// 3. Determine if we should apply idempotency
			isMethodAllowed := manager.IsMethodAllowed(r.Method)
			hasKey := headerKey != ""

			// If no header key and method not allowed, skip early
			if !hasKey && !isMethodAllowed {
				next.ServeHTTP(w, r)
				return
			}

			// 4. Handle Request Body
			var body []byte
			if r.Body != nil {
				var err error
				body, err = io.ReadAll(r.Body)
				if err != nil {
					http.Error(w, `{"error":"failed to read request body"}`, http.StatusBadRequest)
					return
				}
				r.Body.Close()
				r.Body = io.NopCloser(bytes.NewBuffer(body))
			}
			pReq.Body = body

			// 5. Check for cached response
			cachedResp, err := manager.Check(r.Context(), pReq)
			if err != nil {
				if err == idempotency.ErrRequestInProgress {
					http.Error(w, `{"error":"request already in progress"}`, http.StatusConflict)
					return
				}
				if err == idempotency.ErrRequestMismatch {
					http.Error(w, `{"error":"idempotency key reused with different payload"}`, http.StatusUnprocessableEntity)
					return
				}
				// Other errors proceed normally
			}

			// 6. Missing Key Handling (RequireKey check)
			if pReq.IdempotencyKey == "" {
				if manager.Config().RequireKey && isMethodAllowed {
					http.Error(w, `{"error":"idempotency key is required for this request"}`, http.StatusBadRequest)
					return
				}
				next.ServeHTTP(w, r)
				return
			}

			// 7. Return cached response if available
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

			// 8. Acquire lock
			if err := manager.Lock(r.Context(), pReq); err != nil {
				if err == idempotency.ErrRequestInProgress {
					http.Error(w, `{"error":"request already in progress"}`, http.StatusConflict)
					return
				}
				// Handle other errors if necessary
			}

			// 9. Capture response
			recorder := &responseRecorder{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
				body:           &bytes.Buffer{},
			}

			// 10. Process request
			next.ServeHTTP(recorder, r)

			// 11. Store response
			if pReq.IdempotencyKey != "" && recorder.statusCode < 500 {
				resp := &idempotency.Response{
					StatusCode:  recorder.statusCode,
					Headers:     recorder.Header().Clone(),
					Body:        recorder.body.Bytes(),
					ContentType: recorder.Header().Get("Content-Type"),
				}
				_ = manager.Store(r.Context(), pReq.IdempotencyKey, resp)
			} else if pReq.IdempotencyKey != "" {
				_ = manager.Unlock(r.Context(), pReq.IdempotencyKey)
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

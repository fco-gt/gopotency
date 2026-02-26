// Package echo provides Echo framework middleware for idempotency handling.
package echo

import (
	"bytes"
	"io"
	"net/http"

	idempotency "github.com/fco-gt/gopotency"
	"github.com/labstack/echo/v4"
)

// Idempotency returns an Echo middleware that handles idempotency
func Idempotency(manager *idempotency.Manager) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			req := c.Request()

			// 1. Extract potential idempotency key from header
			headerKey := req.Header.Get("Idempotency-Key")

			// 2. Build dummy request for potential auto-generation
			pReq := &idempotency.Request{
				Method:         req.Method,
				Path:           req.URL.Path,
				Headers:        req.Header,
				IdempotencyKey: headerKey,
			}

			// 3. Determine if we should apply idempotency
			isMethodAllowed := manager.IsMethodAllowed(req.Method)
			hasKey := headerKey != ""

			// If no header key and method not allowed, skip early
			if !hasKey && !isMethodAllowed {
				return next(c)
			}

			// 4. Handle Request Body
			var body []byte
			if req.Body != nil {
				var err error
				body, err = io.ReadAll(req.Body)
				if err != nil {
					return echo.NewHTTPError(http.StatusBadRequest, "failed to read request body")
				}
				req.Body.Close()
				req.Body = io.NopCloser(bytes.NewBuffer(body))
			}
			pReq.Body = body

			// 5. Check for cached response
			cachedResp, err := manager.Check(req.Context(), pReq)
			if err != nil {
				if err == idempotency.ErrRequestInProgress {
					return echo.NewHTTPError(http.StatusConflict, "request already in progress")
				}
				if err == idempotency.ErrRequestMismatch {
					return echo.NewHTTPError(http.StatusUnprocessableEntity, "idempotency key reused with different payload")
				}
				// Other errors proceed normally
			}

			// 6. Missing Key Handling (RequireKey check)
			if pReq.IdempotencyKey == "" {
				if manager.Config().RequireKey && isMethodAllowed {
					return echo.NewHTTPError(http.StatusBadRequest, "idempotency key is required for this request")
				}
				return next(c)
			}

			// 7. Return cached response if available
			if cachedResp != nil {
				for key, values := range cachedResp.Headers {
					for _, value := range values {
						c.Response().Header().Add(key, value)
					}
				}
				c.Response().Header().Set("X-Idempotent-Replayed", "true")
				return c.Blob(cachedResp.StatusCode, cachedResp.ContentType, cachedResp.Body)
			}

			// 8. Acquire lock
			if err := manager.Lock(req.Context(), pReq); err != nil {
				if err == idempotency.ErrRequestInProgress {
					return echo.NewHTTPError(http.StatusConflict, "request already in progress")
				}
			}

			// 9. Capture response
			res := c.Response()
			originalWriter := res.Writer
			bodyBuffer := &bytes.Buffer{}
			mw := io.MultiWriter(originalWriter, bodyBuffer)
			res.Writer = &responseWriter{Writer: mw, ResponseWriter: originalWriter}

			// 10. Process request
			err = next(c)

			// Restore original writer
			res.Writer = originalWriter

			// 11. Store response
			if pReq.IdempotencyKey != "" && res.Status < 500 {
				resp := &idempotency.Response{
					StatusCode:  res.Status,
					Headers:     res.Header().Clone(),
					Body:        bodyBuffer.Bytes(),
					ContentType: res.Header().Get("Content-Type"),
				}
				_ = manager.Store(req.Context(), pReq.IdempotencyKey, resp)
			} else if pReq.IdempotencyKey != "" {
				_ = manager.Unlock(req.Context(), pReq.IdempotencyKey)
			}

			return err
		}
	}
}

type responseWriter struct {
	io.Writer
	http.ResponseWriter
}

func (w *responseWriter) Write(b []byte) (int, error) {
	return w.Writer.Write(b)
}

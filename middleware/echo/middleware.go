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

			// Skip if method is not allowed
			if !manager.IsMethodAllowed(req.Method) {
				return next(c)
			}

			// Read body
			body, err := io.ReadAll(req.Body)
			if err != nil {
				return echo.NewHTTPError(http.StatusBadRequest, "failed to read request body")
			}

			// Restore body for downstream handlers
			req.Body = io.NopCloser(bytes.NewBuffer(body))

			// Build request object
			pReq := &idempotency.Request{
				Method:  req.Method,
				Path:    req.URL.Path,
				Headers: req.Header,
				Body:    body,
			}

			// Extract idempotency key from header if present
			if key := req.Header.Get("Idempotency-Key"); key != "" {
				pReq.IdempotencyKey = key
			}

			// Check for cached response
			cachedResp, err := manager.Check(req.Context(), pReq)
			if err != nil {
				if err == idempotency.ErrRequestInProgress {
					return echo.NewHTTPError(http.StatusConflict, "request with this idempotency key is already being processed")
				}

				if err == idempotency.ErrRequestMismatch {
					return echo.NewHTTPError(http.StatusUnprocessableEntity, "request with same idempotency key has different content")
				}

				// Other errors, proceed without idempotency
				return next(c)
			}

			// Return cached response if available
			if cachedResp != nil {
				for key, values := range cachedResp.Headers {
					for _, value := range values {
						c.Response().Header().Add(key, value)
					}
				}
				c.Response().Header().Set("X-Idempotent-Replayed", "true")
				return c.Blob(cachedResp.StatusCode, cachedResp.ContentType, cachedResp.Body)
			}

			// Acquire lock if we have an idempotency key
			if pReq.IdempotencyKey != "" {
				if err := manager.Lock(req.Context(), pReq); err != nil {
					if err == idempotency.ErrRequestInProgress {
						return echo.NewHTTPError(http.StatusConflict, "request with this idempotency key is already being processed")
					}
					// Other errors, proceed without idempotency
				}
			}

			// Capture response
			res := c.Response()
			originalWriter := res.Writer
			bodyBuffer := &bytes.Buffer{}
			mw := io.MultiWriter(originalWriter, bodyBuffer)
			res.Writer = &responseWriter{Writer: mw, ResponseWriter: originalWriter}

			// Process request
			err = next(c)

			// Restore original writer
			res.Writer = originalWriter

			// Store response if we have an idempotency key and processing succeeded
			if pReq.IdempotencyKey != "" && res.Status < 500 {
				resp := &idempotency.Response{
					StatusCode:  res.Status,
					Headers:     res.Header().Clone(),
					Body:        bodyBuffer.Bytes(),
					ContentType: res.Header().Get("Content-Type"),
				}

				_ = manager.Store(req.Context(), pReq.IdempotencyKey, resp)
			} else if pReq.IdempotencyKey != "" {
				// Unlock on error
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

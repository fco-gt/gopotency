// Package fiber provides Fiber framework middleware for idempotency handling.
package fiber

import (
	"net/http"

	idempotency "github.com/fco-gt/gopotency"
	"github.com/gofiber/fiber/v2"
)

// Idempotency returns a Fiber middleware that handles idempotency
func Idempotency(manager *idempotency.Manager) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Skip if method is not allowed
		if !manager.IsMethodAllowed(c.Method()) {
			return c.Next()
		}

		// Build request object
		pReq := &idempotency.Request{
			Method:  c.Method(),
			Path:    c.Path(),
			Headers: make(map[string][]string),
			Body:    c.Body(),
		}

		// Copy headers
		c.Request().Header.VisitAll(func(key, value []byte) {
			k := string(key)
			pReq.Headers[k] = append(pReq.Headers[k], string(value))
		})

		// Extract idempotency key from header if present
		if key := c.Get("Idempotency-Key"); key != "" {
			pReq.IdempotencyKey = key
		}

		// Check for cached response
		cachedResp, err := manager.Check(c.Context(), pReq)
		if err != nil {
			if err == idempotency.ErrRequestInProgress {
				return c.Status(http.StatusConflict).JSON(fiber.Map{
					"error": "request with this idempotency key is already being processed",
				})
			}

			if err == idempotency.ErrRequestMismatch {
				return c.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{
					"error": "request with same idempotency key has different content",
				})
			}

			// Other errors, proceed without idempotency
			return c.Next()
		}

		// Return cached response if available
		if cachedResp != nil {
			for key, values := range cachedResp.Headers {
				for _, value := range values {
					c.Set(key, value)
				}
			}
			c.Set("X-Idempotent-Replayed", "true")
			c.Status(cachedResp.StatusCode)
			if cachedResp.ContentType != "" {
				c.Set(fiber.HeaderContentType, cachedResp.ContentType)
			}
			return c.Send(cachedResp.Body)
		}

		// Acquire lock if we have an idempotency key
		if pReq.IdempotencyKey != "" {
			if err := manager.Lock(c.Context(), pReq); err != nil {
				if err == idempotency.ErrRequestInProgress {
					return c.Status(http.StatusConflict).JSON(fiber.Map{
						"error": "request with this idempotency key is already being processed",
					})
				}
				// Other errors, proceed without idempotency
			}
		}

		// Process request
		err = c.Next()

		// Store response if we have an idempotency key and processing succeeded
		if pReq.IdempotencyKey != "" && c.Response().StatusCode() < 500 {
			headers := make(map[string][]string)
			c.Response().Header.VisitAll(func(key, value []byte) {
				k := string(key)
				headers[k] = append(headers[k], string(value))
			})

			resp := &idempotency.Response{
				StatusCode:  c.Response().StatusCode(),
				Headers:     headers,
				Body:        c.Response().Body(),
				ContentType: string(c.Response().Header.Peek(fiber.HeaderContentType)),
			}

			_ = manager.Store(c.Context(), pReq.IdempotencyKey, resp)
		} else if pReq.IdempotencyKey != "" {
			// Unlock on error
			_ = manager.Unlock(c.Context(), pReq.IdempotencyKey)
		}

		return err
	}
}

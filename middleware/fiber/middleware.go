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
		// 1. Extract potential idempotency key from header
		headerKey := c.Get("Idempotency-Key")

		// 2. Build dummy request
		pReq := &idempotency.Request{
			Method:         c.Method(),
			Path:           c.Path(),
			Headers:        make(map[string][]string),
			Body:           c.Body(),
			IdempotencyKey: headerKey,
		}

		// Copy headers
		c.Request().Header.VisitAll(func(key, value []byte) {
			k := string(key)
			pReq.Headers[k] = append(pReq.Headers[k], string(value))
		})

		// 3. Determine if we should apply idempotency
		isMethodAllowed := manager.IsMethodAllowed(c.Method())
		hasKey := headerKey != ""

		// If no header key and method not allowed, skip early
		if !hasKey && !isMethodAllowed {
			return c.Next()
		}

		// 4. Check for cached response
		cachedResp, err := manager.Check(c.Context(), pReq)
		if err != nil {
			if err == idempotency.ErrRequestInProgress {
				return c.Status(http.StatusConflict).JSON(fiber.Map{"error": "request already in progress"})
			}
			if err == idempotency.ErrRequestMismatch {
				return c.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"error": "idempotency key reused with different payload"})
			}
			// Other errors proceed normally
		}

		// 5. Missing Key Handling (RequireKey check)
		if pReq.IdempotencyKey == "" {
			if manager.Config().RequireKey && isMethodAllowed {
				return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "idempotency key is required for this request"})
			}
			return c.Next()
		}

		// 6. Return cached response if available
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

		// 7. Acquire lock
		if err := manager.Lock(c.Context(), pReq); err != nil {
			if err == idempotency.ErrRequestInProgress {
				return c.Status(http.StatusConflict).JSON(fiber.Map{"error": "request already in progress"})
			}
		}

		// 8. Process request
		err = c.Next()

		// 9. Store response
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
			_ = manager.Unlock(c.Context(), pReq.IdempotencyKey)
		}

		return err
	}
}

// Package idempotency provides a flexible, framework-agnostic solution for handling
// idempotency in HTTP APIs.
//
// Idempotency ensures that performing the same request multiple times produces the same
// result as performing it once, without additional side effects. This is crucial for
// payment processing, resource creation, and other critical operations.
//
// # Quick Start
//
// Create a manager with in-memory storage:
//
//	import (
//		"github.com/fco-gt/gopotency"
//		"github.com/fco-gt/gopotency/storage/memory"
//		"time"
//	)
//
//	store := memory.NewMemoryStorage()
//	manager, err := idempotency.NewManager(idempotency.Config{
//		Storage: store,
//		TTL:     24 * time.Hour,
//	})
//
// Use with Gin:
//
//	import ginmw "github.com/fco-gt/gopotency/middleware/gin"
//
//	router := gin.Default()
//	router.Use(ginmw.Idempotency(manager))
//
// Use with standard library:
//
//	import httpmw "github.com/fco-gt/gopotency/middleware/http"
//
//	mux := http.NewServeMux()
//	handler := httpmw.Idempotency(manager)(yourHandler)
//
// # Key Strategies
//
// The package supports multiple key generation strategies:
//
//   - HeaderBased: Extracts key from request header (default: "Idempotency-Key")
//   - BodyHash: Generates key from request content hash
//   - Composite: Tries header first, falls back to body hash
//
// # Storage Backends
//
//   - memory: In-memory storage (development/testing)
//   - redis: Redis-backed storage (coming soon)
//
// # Configuration
//
// Customize behavior with Config options:
//
//	config := idempotency.Config{
//		Storage:        store,
//		TTL:            24 * time.Hour,
//		LockTimeout:    5 * time.Minute,
//		KeyStrategy:    key.HeaderBased("Idempotency-Key"),
//		AllowedMethods: []string{"POST", "PUT", "PATCH", "DELETE"},
//	}
package idempotency

const (
	// Version is the package version
	Version = "0.1.0"

	// DefaultHeaderName is the default header name for idempotency keys
	DefaultHeaderName = "Idempotency-Key"
)

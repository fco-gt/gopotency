// Package storage defines the interface for idempotency record storage backends.
//
// Available implementations:
//   - memory: In-memory storage (development/testing)
//   - redis: Redis-backed storage (production)
//   - postgres: PostgreSQL-backed storage (production with persistence)
package storage

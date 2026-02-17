package idempotency

import (
	"context"
	"time"
)

// Config holds the configuration for the idempotency manager
type Config struct {
	// Storage is the backend storage implementation (required)
	Storage Storage

	// TTL is the time-to-live for idempotency records
	// Default: 24 hours
	TTL time.Duration

	// LockTimeout is the maximum time a lock can be held
	// This prevents deadlocks if a server crashes while processing
	// Default: 5 minutes
	LockTimeout time.Duration

	// KeyGenerator is the strategy for generating idempotency keys
	// Default: HeaderBased("Idempotency-Key")
	KeyGenerator KeyGenerator

	// RequestHasher computes a hash of the request for validation
	// Default: BodyHasher
	RequestHasher RequestHasher

	// AllowedMethods specifies which HTTP methods should have idempotency applied
	// Default: ["POST", "PUT", "PATCH", "DELETE"]
	AllowedMethods []string

	// ErrorHandler is called when an error occurs, allowing custom error responses
	// Default: returns standard error responses
	ErrorHandler func(error) (statusCode int, body interface{})

	// OnCacheHit is called when a cached response is returned (optional)
	OnCacheHit func(key string)

	// OnCacheMiss is called when no cached response is found (optional)
	OnCacheMiss func(key string)

	// OnLockConflict is called when a request is already in progress (optional)
	OnLockConflict func(key string)
}

// setDefaults sets default values for unspecified config options
func (c *Config) setDefaults() {
	if c.TTL == 0 {
		c.TTL = 24 * time.Hour
	}

	if c.LockTimeout == 0 {
		c.LockTimeout = 5 * time.Minute
	}

	if c.AllowedMethods == nil {
		c.AllowedMethods = []string{"POST", "PUT", "PATCH", "DELETE"}
	}
}

// validate checks if the configuration is valid
func (c *Config) validate() error {
	if c.Storage == nil {
		return ErrStorageNotConfigured
	}

	return nil
}

// Storage is the interface for storing and retrieving idempotency records
type Storage interface {
	// Get retrieves an idempotency record by key
	Get(ctx context.Context, key string) (*Record, error)

	// Set stores an idempotency record
	Set(ctx context.Context, record *Record, ttl time.Duration) error

	// Delete removes an idempotency record
	Delete(ctx context.Context, key string) error

	// Exists checks if a record exists
	Exists(ctx context.Context, key string) (bool, error)

	// TryLock attempts to acquire a lock for the given key
	// Returns true if lock was acquired, false if already locked
	TryLock(ctx context.Context, key string, ttl time.Duration) (bool, error)

	// Unlock releases a lock for the given key
	Unlock(ctx context.Context, key string) error

	// Close closes the storage connection
	Close() error
}

// KeyGenerator is the interface for generating idempotency keys
type KeyGenerator interface {
	// Generate generates an idempotency key from the request
	// Returns empty string if no key can be generated
	Generate(req *Request) (string, error)
}

// RequestHasher is the interface for hashing requests
type RequestHasher interface {
	// Hash computes a hash of the request for validation
	Hash(req *Request) (string, error)
}

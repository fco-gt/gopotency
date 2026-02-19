package idempotency

import (
	"context"
	"slices"
	"time"
)

// Manager handles idempotency checks and response caching
type Manager struct {
	config Config
}

// NewManager creates a new idempotency manager with the given configuration
func NewManager(config Config) (*Manager, error) {
	// Set defaults
	config.setDefaults()

	// Validate configuration
	if err := config.validate(); err != nil {
		return nil, err
	}

	return &Manager{
		config: config,
	}, nil
}

// Check verifies if a request should be processed or if a cached response exists
// Returns:
// - *CachedResponse: if the request was already processed successfully
// - error: ErrRequestInProgress if currently being processed, or other errors
func (m *Manager) Check(ctx context.Context, req *Request) (*CachedResponse, error) {
	// Generate idempotency key if not already set
	if req.IdempotencyKey == "" {
		if m.config.KeyStrategy != nil {
			key, err := m.config.KeyStrategy.Generate(req)
			if err != nil {
				return nil, err
			}
			req.IdempotencyKey = key
		}

		// If still no key, return (idempotency not applicable)
		if req.IdempotencyKey == "" {
			return nil, nil
		}
	}

	// Check if record exists
	record, err := m.config.Storage.Get(ctx, req.IdempotencyKey)
	if err != nil || record == nil {
		// No record found or storage error - this is a new request
		return nil, nil
	}

	// Validate request hash if hasher is configured
	if m.config.RequestHasher != nil {
		reqHash, err := m.config.RequestHasher.Hash(req)
		if err == nil && record.RequestHash != "" && record.RequestHash != reqHash {
			return nil, ErrRequestMismatch
		}
	}

	// Check record status
	switch record.Status {
	case StatusPending:
		// Request is currently being processed
		if m.config.OnLockConflict != nil {
			m.config.OnLockConflict(req.IdempotencyKey)
		}
		return nil, ErrRequestInProgress

	case StatusCompleted:
		// Return cached response
		if m.config.OnCacheHit != nil {
			m.config.OnCacheHit(req.IdempotencyKey)
		}
		return record.Response, nil

	case StatusFailed:
		// Failed requests can be retried (treat as new)
		return nil, nil

	default:
		// Unknown status, treat as new
		return nil, nil
	}
}

// Lock attempts to acquire a lock for processing the request
func (m *Manager) Lock(ctx context.Context, req *Request) error {
	if req.IdempotencyKey == "" {
		return ErrNoIdempotencyKey
	}

	// Compute request hash
	var reqHash string
	if m.config.RequestHasher != nil {
		hash, err := m.config.RequestHasher.Hash(req)
		if err != nil {
			return err
		}
		reqHash = hash
	}

	// Create pending record
	record := &Record{
		Key:         req.IdempotencyKey,
		RequestHash: reqHash,
		Status:      StatusPending,
		CreatedAt:   time.Now(),
		ExpiresAt:   time.Now().Add(m.config.TTL),
	}

	// Try to acquire lock
	locked, err := m.config.Storage.TryLock(ctx, req.IdempotencyKey, m.config.LockTimeout)
	if err != nil {
		return NewStorageError("trylock", err)
	}

	if !locked {
		// Lock already held by another request
		return ErrRequestInProgress
	}

	// Store pending record
	if err := m.config.Storage.Set(ctx, record, m.config.TTL); err != nil {
		// Try to unlock if set fails
		_ = m.config.Storage.Unlock(ctx, req.IdempotencyKey)
		return NewStorageError("set", err)
	}

	if m.config.OnCacheMiss != nil {
		m.config.OnCacheMiss(req.IdempotencyKey)
	}

	return nil
}

// Store saves the response for a successfully processed request
func (m *Manager) Store(ctx context.Context, key string, resp *Response) error {
	if key == "" {
		return ErrNoIdempotencyKey
	}

	// Get existing record to preserve request hash
	record, err := m.config.Storage.Get(ctx, key)
	if err != nil || record == nil {
		// Create new record if not found or on error
		record = &Record{
			Key:       key,
			CreatedAt: time.Now(),
		}
	}

	// Update record with response
	record.Status = StatusCompleted
	record.Response = resp.ToCachedResponse()
	record.ExpiresAt = time.Now().Add(m.config.TTL)

	// Store updated record
	if err := m.config.Storage.Set(ctx, record, m.config.TTL); err != nil {
		return NewStorageError("set", err)
	}

	// Release lock
	if err := m.config.Storage.Unlock(ctx, key); err != nil {
		// Log error but don't fail the operation
		// The lock will eventually expire
	}

	return nil
}

// Unlock releases the lock for a request (typically called on error)
func (m *Manager) Unlock(ctx context.Context, key string) error {
	if key == "" {
		return nil
	}

	if err := m.config.Storage.Unlock(ctx, key); err != nil {
		return NewStorageError("unlock", err)
	}

	return nil
}

// IsMethodAllowed checks if idempotency should be applied to the given HTTP method
func (m *Manager) IsMethodAllowed(method string) bool {
	if len(m.config.AllowedMethods) == 0 {
		return false
	}

	return slices.Contains(m.config.AllowedMethods, method)
}

// Close closes the manager and underlying storage
func (m *Manager) Close() error {
	if m.config.Storage != nil {
		return m.config.Storage.Close()
	}
	return nil
}

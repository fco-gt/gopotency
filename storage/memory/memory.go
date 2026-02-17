// Package memory provides an in-memory storage implementation for idempotency records.
//
// This storage is suitable for development, testing, and single-instance applications.
// For production distributed systems, use Redis or PostgreSQL storage instead.
package memory

import (
	"context"
	"sync"
	"time"

	"github.com/fco-gt/gopotency"
)

// Storage is an in-memory implementation of idempotency.Storage
type Storage struct {
	mu      sync.RWMutex
	records map[string]*idempotency.Record
	locks   map[string]time.Time
}

// NewMemoryStorage creates a new in-memory storage instance
func NewMemoryStorage() *Storage {
	s := &Storage{
		records: make(map[string]*idempotency.Record),
		locks:   make(map[string]time.Time),
	}

	// Start cleanup goroutine
	go s.cleanup()

	return s
}

// Get retrieves an idempotency record by key
func (s *Storage) Get(ctx context.Context, key string) (*idempotency.Record, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	record, exists := s.records[key]
	if !exists {
		return nil, idempotency.NewStorageError("get", idempotency.ErrStorageOperation)
	}

	// Check if expired
	if time.Now().After(record.ExpiresAt) {
		return nil, idempotency.NewStorageError("get", idempotency.ErrStorageOperation)
	}

	return record, nil
}

// Set stores an idempotency record
func (s *Storage) Set(ctx context.Context, record *idempotency.Record, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Set expiration if not already set
	if record.ExpiresAt.IsZero() {
		record.ExpiresAt = time.Now().Add(ttl)
	}

	s.records[record.Key] = record
	return nil
}

// Delete removes an idempotency record
func (s *Storage) Delete(ctx context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.records, key)
	delete(s.locks, key)
	return nil
}

// Exists checks if a record exists
func (s *Storage) Exists(ctx context.Context, key string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	record, exists := s.records[key]
	if !exists {
		return false, nil
	}

	// Check if expired
	if time.Now().After(record.ExpiresAt) {
		return false, nil
	}

	return true, nil
}

// TryLock attempts to acquire a lock for the given key
func (s *Storage) TryLock(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if lock exists and is not expired
	if lockExpiry, exists := s.locks[key]; exists {
		if time.Now().Before(lockExpiry) {
			return false, nil // Lock already held
		}
		// Lock expired, can be acquired
	}

	// Acquire lock
	s.locks[key] = time.Now().Add(ttl)
	return true, nil
}

// Unlock releases a lock for the given key
func (s *Storage) Unlock(ctx context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.locks, key)
	return nil
}

// Close closes the storage (no-op for memory storage)
func (s *Storage) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Clear all data
	s.records = make(map[string]*idempotency.Record)
	s.locks = make(map[string]time.Time)

	return nil
}

// cleanup periodically removes expired records and locks
func (s *Storage) cleanup() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		s.mu.Lock()

		now := time.Now()

		// Remove expired records
		for key, record := range s.records {
			if now.After(record.ExpiresAt) {
				delete(s.records, key)
			}
		}

		// Remove expired locks
		for key, expiry := range s.locks {
			if now.After(expiry) {
				delete(s.locks, key)
			}
		}

		s.mu.Unlock()
	}
}

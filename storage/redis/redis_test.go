package redis

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	idempotency "github.com/fco-gt/gopotency"
	"github.com/redis/go-redis/v9"
)

func TestRedisStorage_CompleteFlow(t *testing.T) {
	// SETUP: Initialize miniredis, a temporary in-memory Redis server for testing
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Failed to start miniredis: %v", err)
	}
	defer mr.Close()

	// Initialize the Redis client pointing to the miniredis instance
	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	storage := &RedisStorage{client: client}
	ctx := context.Background()

	key := "test-idempotency-key"
	record := &idempotency.Record{
		Key:       key,
		Status:    idempotency.StatusCompleted,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(time.Hour),
	}

	// Sub-test: Saving a record
	t.Run("SaveRecord", func(t *testing.T) {
		err := storage.Set(ctx, record, time.Hour)
		if err != nil {
			t.Fatalf("Set operation failed: %v", err)
		}
	})

	// Sub-test: Retrieving the saved record
	t.Run("GetRecord", func(t *testing.T) {
		got, err := storage.Get(ctx, key)
		if err != nil {
			t.Fatalf("Get operation failed: %v", err)
		}
		if got == nil || got.Key != key {
			t.Errorf("Expected key %s, but got %v", key, got)
		}
	})

	// Sub-test: Distributed Locking logic (Concurrency control)
	t.Run("LocksAndConcurrency", func(t *testing.T) {
		lockKey := "concurrency-key"
		ttl := time.Second * 5

		// 1. Acquire the first lock (should succeed)
		locked, err := storage.TryLock(ctx, lockKey, ttl)
		if err != nil {
			t.Fatalf("TryLock failed with error: %v", err)
		}
		if !locked {
			t.Error("Expected to acquire lock, but it was already held")
		}

		// 2. Attempt to acquire the same lock again (should fail)
		lockedAgain, _ := storage.TryLock(ctx, lockKey, ttl)
		if lockedAgain {
			t.Error("Lock conflict: secondary process acquired an existing lock")
		}

		// 3. Release the lock
		err = storage.Unlock(ctx, lockKey)
		if err != nil {
			t.Fatalf("Unlock operation failed: %v", err)
		}

		// 4. Acquire the lock again after release (should succeed)
		lockedPostUnlock, _ := storage.TryLock(ctx, lockKey, ttl)
		if !lockedPostUnlock {
			t.Error("Failed to re-acquire lock after it was released")
		}
	})

	// Sub-test: Deleting a record
	t.Run("DeleteRecord", func(t *testing.T) {
		err := storage.Delete(ctx, key)
		if err != nil {
			t.Fatalf("Delete operation failed: %v", err)
		}

		// Verify the record no longer exists
		exists, _ := storage.Exists(ctx, key)
		if exists {
			t.Error("Data leakage: record still exists after deletion")
		}
	})
}

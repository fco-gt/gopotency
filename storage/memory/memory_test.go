package memory

import (
	"context"
	"testing"
	"time"

	idempotency "github.com/fco-gt/gopotency"
)

func TestMemoryStorage_CompleteFlow(t *testing.T) {
	// SETUP: Initialize in-memory storage
	store := NewMemoryStorage()
	defer store.Close()
	ctx := context.Background()

	// Test data
	key := "test-memory-key"
	record := &idempotency.Record{
		Key:       key,
		Status:    idempotency.StatusCompleted,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(time.Hour),
	}

	// Sub-test: Saving a record
	t.Run("SaveRecord", func(t *testing.T) {
		err := store.Set(ctx, record, time.Hour)
		if err != nil {
			t.Fatalf("Failed to save record: %v", err)
		}
	})

	// Sub-test: Retrieving the record
	t.Run("GetRecord", func(t *testing.T) {
		got, err := store.Get(ctx, key)
		if err != nil {
			t.Fatalf("Failed to get record: %v", err)
		}
		if got.Key != key {
			t.Errorf("Key mismatch: expected %s, got %s", key, got.Key)
		}
	})

	// Sub-test: Locking logic
	t.Run("Locks", func(t *testing.T) {
		lockKey := "lock-key"
		ttl := time.Second * 2

		// 1. Acquire lock
		locked, err := store.TryLock(ctx, lockKey, ttl)
		if err != nil {
			t.Fatalf("TryLock failed: %v", err)
		}
		if !locked {
			t.Error("Should have acquired the lock")
		}

		// 2. Try to acquire again (should fail)
		lockedAgain, _ := store.TryLock(ctx, lockKey, ttl)
		if lockedAgain {
			t.Error("Lock should be already held")
		}

		// 3. Unlock
		err = store.Unlock(ctx, lockKey)
		if err != nil {
			t.Fatalf("Unlock failed: %v", err)
		}

		// 4. Try again after unlock (should succeed)
		lockedPostUnlock, _ := store.TryLock(ctx, lockKey, ttl)
		if !lockedPostUnlock {
			t.Error("Should have re-acquired the lock after unlocking")
		}
	})

	// Sub-test: Deletion
	t.Run("DeleteRecord", func(t *testing.T) {
		err := store.Delete(ctx, key)
		if err != nil {
			t.Fatalf("Delete failed: %v", err)
		}

		exists, _ := store.Exists(ctx, key)
		if exists {
			t.Error("Record should have been deleted")
		}
	})
}

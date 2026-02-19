package gorm

import (
	"context"
	"testing"
	"time"

	idempotency "github.com/fco-gt/gopotency"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestGormStorage_CompleteFlow(t *testing.T) {
	// SETUP: Initialize GORM with an in-memory SQLite database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		// Disable logging for cleaner test output
		Logger: nil,
	})
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}

	// Run auto-migration for test models
	err = db.AutoMigrate(&IdempotencyRecord{}, &IdempotencyLock{})
	if err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	storage := NewGormStorage(db)
	ctx := context.Background()

	key := "test-gorm-key"
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

	// Sub-test: Distributed Locking logic
	t.Run("LocksAndConcurrency", func(t *testing.T) {
		lockKey := "gorm-lock-key"
		ttl := time.Second * 5

		// 1. Acquire the first lock
		locked, err := storage.TryLock(ctx, lockKey, ttl)
		if err != nil {
			t.Fatalf("TryLock failed: %v", err)
		}
		if !locked {
			t.Error("Expected to acquire lock, but it was already held")
		}

		// 2. Attempt to acquire the same lock again (should fail)
		lockedAgain, _ := storage.TryLock(ctx, lockKey, ttl)
		if lockedAgain {
			t.Error("Lock conflict: acquired an existing lock")
		}

		// 3. Release the lock
		err = storage.Unlock(ctx, lockKey)
		if err != nil {
			t.Fatalf("Unlock operation failed: %v", err)
		}

		// 4. Acquire the lock again after release
		lockedPostUnlock, _ := storage.TryLock(ctx, lockKey, ttl)
		if !lockedPostUnlock {
			t.Error("Failed to re-acquire lock after it was released")
		}
	})

	// Sub-test: Exists check
	t.Run("Exists", func(t *testing.T) {
		exists, err := storage.Exists(ctx, key)
		if err != nil {
			t.Fatalf("Exists check failed: %v", err)
		}
		if !exists {
			t.Error("Expected record to exist")
		}

		missing, _ := storage.Exists(ctx, "non-existent")
		if missing {
			t.Error("Expected record to be missing")
		}
	})

	// Sub-test: Deleting a record
	t.Run("DeleteRecord", func(t *testing.T) {
		err := storage.Delete(ctx, key)
		if err != nil {
			t.Fatalf("Delete operation failed: %v", err)
		}

		// Verify deletion
		exists, _ := storage.Exists(ctx, key)
		if exists {
			t.Error("Record still exists after deletion")
		}
	})
}

package sql

import (
	"context"
	"database/sql"
	"testing"
	"time"

	idempotency "github.com/fco-gt/gopotency"
	_ "modernc.org/sqlite" // Pure Go SQLite driver for tests
)

func TestSQLStorage(t *testing.T) {
	// 1. Setup in-memory SQLite
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	defer db.Close()

	// 2. Create tables
	tableName := "test_idempotency"
	_, err = db.Exec(`CREATE TABLE test_idempotency (key TEXT PRIMARY KEY, data BLOB, expires_at DATETIME)`)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}
	_, err = db.Exec(`CREATE TABLE test_idempotency_locks (key TEXT PRIMARY KEY, expires_at DATETIME)`)
	if err != nil {
		t.Fatalf("failed to create locks table: %v", err)
	}

	store := NewSQLStorage(db, tableName)
	ctx := context.Background()

	// 3. Test Set & Get
	t.Run("SetAndGet", func(t *testing.T) {
		record := &idempotency.Record{
			Key:       "key1",
			Status:    idempotency.StatusCompleted,
			CreatedAt: time.Now(),
		}
		err := store.Set(ctx, record, time.Hour)
		if err != nil {
			t.Fatalf("Set failed: %v", err)
		}

		got, err := store.Get(ctx, "key1")
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}
		if got == nil || got.Key != "key1" {
			t.Errorf("expected key1, got %v", got)
		}
	})

	// 4. Test TryLock & Unlock
	t.Run("Locking", func(t *testing.T) {
		locked, err := store.TryLock(ctx, "lock1", time.Second)
		if err != nil {
			t.Fatalf("TryLock failed: %v", err)
		}
		if !locked {
			t.Fatal("expected to acquire lock")
		}

		// Try again
		locked2, _ := store.TryLock(ctx, "lock1", time.Second)
		if locked2 {
			t.Fatal("expected lock to be already held")
		}

		err = store.Unlock(ctx, "lock1")
		if err != nil {
			t.Fatalf("Unlock failed: %v", err)
		}

		locked3, _ := store.TryLock(ctx, "lock1", time.Second)
		if !locked3 {
			t.Fatal("expected to re-acquire lock after unlock")
		}
	})

	// 5. Test Exists
	t.Run("Exists", func(t *testing.T) {
		exists, err := store.Exists(ctx, "key1")
		if err != nil {
			t.Fatalf("Exists failed: %v", err)
		}
		if !exists {
			t.Error("expected key1 to exist")
		}

		exists2, _ := store.Exists(ctx, "non-existent")
		if exists2 {
			t.Error("expected non-existent key to not exist")
		}
	})

	// 6. Test Delete
	t.Run("Delete", func(t *testing.T) {
		err := store.Delete(ctx, "key1")
		if err != nil {
			t.Fatalf("Delete failed: %v", err)
		}

		exists, _ := store.Exists(ctx, "key1")
		if exists {
			t.Error("expected key1 to be deleted")
		}
	})

	// 8. Test Expiration
	t.Run("Expiration", func(t *testing.T) {
		record := &idempotency.Record{
			Key:       "expired-key",
			Status:    idempotency.StatusCompleted,
			ExpiresAt: time.Now().Add(-time.Minute),
		}
		_ = store.Set(ctx, record, -time.Minute)

		got, err := store.Get(ctx, "expired-key")
		if err != nil {
			t.Fatalf("Get unexpected error: %v", err)
		}
		if got != nil {
			t.Error("Expected nil for expired record")
		}
	})

	// 7. Test Close
	t.Run("Close", func(t *testing.T) {
		if err := store.Close(); err != nil {
			t.Errorf("expected nil error on close, got %v", err)
		}
	})

	// 9. Test DB Error
	t.Run("DBError", func(t *testing.T) {
		dbLow, _ := sql.Open("sqlite", ":memory:")
		dbLow.Close() // Close immediately to cause errors
		storeLow := NewSQLStorage(dbLow, "err_table")

		_, err := storeLow.Get(ctx, "any")
		if err == nil {
			t.Error("Expected error for closed database")
		}
	})
}

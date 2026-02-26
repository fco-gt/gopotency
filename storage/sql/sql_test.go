package sql

import (
	"context"
	"database/sql"
	"testing"
	"time"

	idempotency "github.com/fco-gt/gopotency"
	_ "github.com/mattn/go-sqlite3" // Using sqlite3 for testing
)

func TestSQLStorage(t *testing.T) {
	// 1. Setup in-memory SQLite
	db, err := sql.Open("sqlite3", ":memory:")
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
}

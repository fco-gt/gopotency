package idempotency

import (
	"context"
	"errors"
	"testing"
	"time"
)

// TestManager_Config simply ensures we return the stored config.
func TestManager_Config(t *testing.T) {
	cfg := Config{
		AllowedMethods: []string{"POST"},
	}
	m, err := NewManager(Config{
		Storage:        &MockStorage{},
		AllowedMethods: cfg.AllowedMethods,
	})
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	got := m.Config()
	if len(got.AllowedMethods) != 1 || got.AllowedMethods[0] != "POST" {
		t.Fatalf("unexpected config returned: %+v", got.AllowedMethods)
	}
}

func TestManager_Check_RequestHasherMismatchAndHooks(t *testing.T) {
	ctx := context.Background()

	// Record without RequestHash so mismatch logic doesn't apply yet
	record := &Record{
		Key:    "k",
		Status: StatusPending,
		Response: &CachedResponse{
			StatusCode: 200,
		},
		ExpiresAt: time.Now().Add(time.Hour),
	}

	var lockConflictCalled, cacheHitCalled bool

	m, err := NewManager(Config{
		Storage: &MockStorage{
			GetFunc: func(ctx context.Context, key string) (*Record, error) {
				return record, nil
			},
		},
		RequestHasher: RequestHasherFunc(func(req *Request) (string, error) {
			// Different hash to trigger mismatch when status is not pending
			return "different-hash", nil
		}),
		OnLockConflict: func(key string) {
			lockConflictCalled = true
		},
		OnCacheHit: func(key string) {
			cacheHitCalled = true
		},
	})
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	// 1) Pending status should trigger ErrRequestInProgress and OnLockConflict
	req := &Request{Method: "POST", Path: "/", IdempotencyKey: "k"}
	_, err = m.Check(ctx, req)
	if !errors.Is(err, ErrRequestInProgress) {
		t.Fatalf("expected ErrRequestInProgress, got %v", err)
	}
	if !lockConflictCalled {
		t.Fatalf("expected OnLockConflict to be called")
	}

	// 2) Completed status should bypass mismatch when hasher errors,
	//    and call OnCacheHit + return cached response.
	record.Status = StatusCompleted
	lockConflictCalled = false

	m.config.RequestHasher = RequestHasherFunc(func(req *Request) (string, error) {
		return "", errors.New("hash failure")
	})

	resp, err := m.Check(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil || resp.StatusCode != 200 {
		t.Fatalf("expected cached response with 200, got %+v", resp)
	}
	if !cacheHitCalled {
		t.Fatalf("expected OnCacheHit to be called")
	}

	// 3) Failed status should be treated as new (nil response, nil error)
	record.Status = StatusFailed
	cacheHitCalled = false

	resp, err = m.Check(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp != nil {
		t.Fatalf("expected nil response for failed status")
	}
	if cacheHitCalled {
		t.Fatalf("did not expect OnCacheHit for failed status")
	}
}

// RequestHasherFunc is a helper to create RequestHasher from a function.
type RequestHasherFunc func(req *Request) (string, error)

func (f RequestHasherFunc) Hash(req *Request) (string, error) {
	return f(req)
}

func TestManager_Lock_ErrorPathsAndHooks(t *testing.T) {
	ctx := context.Background()

	// 1) Missing key
	m1, _ := NewManager(Config{Storage: &MockStorage{}})
	if err := m1.Lock(ctx, &Request{}); !errors.Is(err, ErrNoIdempotencyKey) {
		t.Fatalf("expected ErrNoIdempotencyKey, got %v", err)
	}

	// 2) Hasher error
	m2, _ := NewManager(Config{
		Storage: &MockStorage{},
		RequestHasher: RequestHasherFunc(func(req *Request) (string, error) {
			return "", errors.New("hash error")
		}),
	})
	if err := m2.Lock(ctx, &Request{IdempotencyKey: "k"}); err == nil || err.Error() != "hash error" {
		t.Fatalf("expected hash error, got %v", err)
	}

	// 3) TryLock storage error
	storageErr := errors.New("trylock failed")
	m3, _ := NewManager(Config{
		Storage: &MockStorage{
			TryLockFunc: func(ctx context.Context, k string, t time.Duration) (bool, error) {
				return false, storageErr
			},
		},
	})
	err := m3.Lock(ctx, &Request{IdempotencyKey: "k"})
	if se, ok := err.(*StorageError); !ok || se.Operation != "trylock" || !errors.Is(se.Err, storageErr) {
		t.Fatalf("expected StorageError(trylock), got %T %v", err, err)
	}

	// 4) TryLock returns false -> ErrRequestInProgress
	m4, _ := NewManager(Config{
		Storage: &MockStorage{
			TryLockFunc: func(ctx context.Context, k string, t time.Duration) (bool, error) {
				return false, nil
			},
		},
	})
	err = m4.Lock(ctx, &Request{IdempotencyKey: "k"})
	if !errors.Is(err, ErrRequestInProgress) {
		t.Fatalf("expected ErrRequestInProgress, got %v", err)
	}

	// 5) Set error should wrap and attempt Unlock; OnCacheMiss should still be called only on success.
	setErr := errors.New("set failed")
	var unlocked bool
	var cacheMissCalled bool
	m5, _ := NewManager(Config{
		Storage: &MockStorage{
			TryLockFunc: func(ctx context.Context, k string, t time.Duration) (bool, error) {
				return true, nil
			},
			SetFunc: func(ctx context.Context, r *Record, ttl time.Duration) error {
				return setErr
			},
			UnlockFunc: func(ctx context.Context, key string) error {
				unlocked = true
				return nil
			},
		},
		OnCacheMiss: func(key string) {
			cacheMissCalled = true
		},
	})
	err = m5.Lock(ctx, &Request{IdempotencyKey: "k"})
	if se, ok := err.(*StorageError); !ok || se.Operation != "set" || !errors.Is(se.Err, setErr) {
		t.Fatalf("expected StorageError(set), got %T %v", err, err)
	}
	if !unlocked {
		t.Fatalf("expected Unlock to be called when Set fails")
	}
	if cacheMissCalled {
		t.Fatalf("did not expect OnCacheMiss on failed lock")
	}

	// 6) Successful lock should call OnCacheMiss
	unlocked = false
	cacheMissCalled = false
	m6, _ := NewManager(Config{
		Storage: &MockStorage{
			TryLockFunc: func(ctx context.Context, k string, t time.Duration) (bool, error) {
				return true, nil
			},
			SetFunc: func(ctx context.Context, r *Record, ttl time.Duration) error {
				return nil
			},
		},
		OnCacheMiss: func(key string) {
			cacheMissCalled = true
		},
	})
	if err := m6.Lock(ctx, &Request{IdempotencyKey: "k"}); err != nil {
		t.Fatalf("unexpected error on successful lock: %v", err)
	}
	if !cacheMissCalled {
		t.Fatalf("expected OnCacheMiss to be called on successful lock")
	}
}

func TestManager_Store_ErrorPathsAndUnlock(t *testing.T) {
	ctx := context.Background()

	// 1) Missing key
	m1, _ := NewManager(Config{Storage: &MockStorage{}})
	if err := m1.Store(ctx, "", &Response{}); !errors.Is(err, ErrNoIdempotencyKey) {
		t.Fatalf("expected ErrNoIdempotencyKey, got %v", err)
	}

	// 2) Get error should create new record but not fail
	getErr := errors.New("get failed")
	var setCalled bool
	m2, _ := NewManager(Config{
		Storage: &MockStorage{
			GetFunc: func(ctx context.Context, key string) (*Record, error) {
				return nil, getErr
			},
			SetFunc: func(ctx context.Context, r *Record, ttl time.Duration) error {
				setCalled = true
				if r.Key != "k" {
					t.Fatalf("expected key k, got %s", r.Key)
				}
				if r.Status != StatusCompleted {
					t.Fatalf("expected StatusCompleted, got %s", r.Status)
				}
				return nil
			},
		},
	})
	if err := m2.Store(ctx, "k", &Response{StatusCode: 201}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !setCalled {
		t.Fatalf("expected Set to be called")
	}

	// 3) Preserve existing RequestHash and wrap Set error
	existing := &Record{
		Key:         "k",
		RequestHash: "hash",
	}
	setErr := errors.New("set failed")
	m3, _ := NewManager(Config{
		Storage: &MockStorage{
			GetFunc: func(ctx context.Context, key string) (*Record, error) {
				return existing, nil
			},
			SetFunc: func(ctx context.Context, r *Record, ttl time.Duration) error {
				if r.RequestHash != "hash" {
					t.Fatalf("expected RequestHash to be preserved, got %s", r.RequestHash)
				}
				return setErr
			},
		},
	})
	err := m3.Store(ctx, "k", &Response{StatusCode: 200})
	if se, ok := err.(*StorageError); !ok || se.Operation != "set" || !errors.Is(se.Err, setErr) {
		t.Fatalf("expected StorageError(set), got %T %v", err, err)
	}
}

func TestManager_Unlock_Behavior(t *testing.T) {
	ctx := context.Background()

	// Empty key should be no-op
	m1, _ := NewManager(Config{Storage: &MockStorage{}})
	if err := m1.Unlock(ctx, ""); err != nil {
		t.Fatalf("expected nil error for empty key, got %v", err)
	}

	// Unlock error should be wrapped
	unlockErr := errors.New("unlock failed")
	m2, _ := NewManager(Config{
		Storage: &MockStorage{
			UnlockFunc: func(ctx context.Context, key string) error {
				return unlockErr
			},
		},
	})
	err := m2.Unlock(ctx, "k")
	if se, ok := err.(*StorageError); !ok || se.Operation != "unlock" || !errors.Is(se.Err, unlockErr) {
		t.Fatalf("expected StorageError(unlock), got %T %v", err, err)
	}
}


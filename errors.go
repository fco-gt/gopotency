package idempotency

import "errors"

// Common errors returned by the idempotency package
var (
	// ErrStorageNotConfigured is returned when no storage backend is configured
	ErrStorageNotConfigured = errors.New("idempotency: storage backend not configured")

	// ErrKeyGenerationFailed is returned when the key generator fails to generate a key
	ErrKeyGenerationFailed = errors.New("idempotency: failed to generate idempotency key")

	// ErrRequestInProgress is returned when a request with the same idempotency key is already being processed
	ErrRequestInProgress = errors.New("idempotency: request with this idempotency key is already in progress")

	// ErrInvalidConfiguration is returned when the configuration is invalid
	ErrInvalidConfiguration = errors.New("idempotency: invalid configuration")

	// ErrRequestMismatch is returned when a duplicate request has different content
	ErrRequestMismatch = errors.New("idempotency: request with same key has different content")

	// ErrStorageOperation is returned when a storage operation fails
	ErrStorageOperation = errors.New("idempotency: storage operation failed")

	// ErrLockTimeout is returned when unable to acquire a lock within the timeout period
	ErrLockTimeout = errors.New("idempotency: lock acquisition timeout")

	// ErrNoIdempotencyKey is returned when no idempotency key could be extracted or generated
	ErrNoIdempotencyKey = errors.New("idempotency: no idempotency key found or generated")
)

// StorageError wraps errors from storage operations
type StorageError struct {
	Operation string
	Err       error
}

func (e *StorageError) Error() string {
	return "idempotency storage " + e.Operation + ": " + e.Err.Error()
}

func (e *StorageError) Unwrap() error {
	return e.Err
}

// NewStorageError creates a new storage error
func NewStorageError(operation string, err error) error {
	return &StorageError{
		Operation: operation,
		Err:       err,
	}
}

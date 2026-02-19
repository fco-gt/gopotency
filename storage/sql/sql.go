// Package sql provides a SQL-based storage backend for gopotency.
// It supports any database compatible with database/sql, such as PostgreSQL, MySQL, or SQLite.
package sql

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	idempotency "github.com/fco-gt/gopotency"
)

// Storage is a SQL implementation of idempotency.Storage
type Storage struct {
	db        *sql.DB
	tableName string
}

// NewSQLStorage creates a new SQL storage instance.
// This implementation is optimized for PostgreSQL and SQLite as it uses
// positional placeholders ($1, $2) and "ON CONFLICT" syntax.
func NewSQLStorage(db *sql.DB, tableName string) *Storage {
	if tableName == "" {
		tableName = "idempotency_records"
	}
	return &Storage{
		db:        db,
		tableName: tableName,
	}
}

// Get retrieves an idempotency record by key
func (s *Storage) Get(ctx context.Context, key string) (*idempotency.Record, error) {
	var data []byte
	var expiresAt time.Time

	query := fmt.Sprintf("SELECT data, expires_at FROM %s WHERE key = $1", s.tableName)
	err := s.db.QueryRowContext(ctx, query, key).Scan(&data, &expiresAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, idempotency.NewStorageError("get", err)
	}

	// Check expiration
	if time.Now().After(expiresAt) {
		_ = s.Delete(ctx, key)
		return nil, nil
	}

	var record idempotency.Record
	if err := json.Unmarshal(data, &record); err != nil {
		return nil, fmt.Errorf("failed to unmarshal record: %w", err)
	}

	return &record, nil
}

// Set stores an idempotency record
func (s *Storage) Set(ctx context.Context, record *idempotency.Record, ttl time.Duration) error {
	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("failed to marshal record: %w", err)
	}

	expiresAt := time.Now().Add(ttl)
	query := fmt.Sprintf(`
		INSERT INTO %s (key, data, expires_at) 
		VALUES ($1, $2, $3) 
		ON CONFLICT (key) DO UPDATE SET data = $2, expires_at = $3`, s.tableName)

	_, err = s.db.ExecContext(ctx, query, record.Key, data, expiresAt)
	if err != nil {
		return idempotency.NewStorageError("set", err)
	}

	return nil
}

// Delete removes an idempotency record
func (s *Storage) Delete(ctx context.Context, key string) error {
	query := fmt.Sprintf("DELETE FROM %s WHERE key = $1", s.tableName)
	_, err := s.db.ExecContext(ctx, query, key)
	if err != nil {
		return idempotency.NewStorageError("delete", err)
	}
	return s.Unlock(ctx, key)
}

// Exists checks if a record exists
func (s *Storage) Exists(ctx context.Context, key string) (bool, error) {
	var exists bool
	query := fmt.Sprintf("SELECT EXISTS(SELECT 1 FROM %s WHERE key = $1 AND expires_at > $2)", s.tableName)
	err := s.db.QueryRowContext(ctx, query, key, time.Now()).Scan(&exists)
	if err != nil {
		return false, idempotency.NewStorageError("exists", err)
	}
	return exists, nil
}

// TryLock attempts to acquire a lock for the given key.
// Note: This is a simplified implementation using a locks table.
// For PostgreSQL, dedicated advisory locks might be better.
func (s *Storage) TryLock(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	expiresAt := time.Now().Add(ttl)

	// Try to insert a lock record. If it exists and is expired, we update it.
	query := fmt.Sprintf(`
		INSERT INTO %s_locks (key, expires_at) 
		VALUES ($1, $2) 
		ON CONFLICT (key) DO UPDATE 
		SET expires_at = $2 
		WHERE %s_locks.expires_at < $3`, s.tableName, s.tableName)

	res, err := s.db.ExecContext(ctx, query, key, expiresAt, time.Now())
	if err != nil {
		return false, idempotency.NewStorageError("trylock", err)
	}

	rows, _ := res.RowsAffected()
	return rows > 0, nil
}

// Unlock releases a lock
func (s *Storage) Unlock(ctx context.Context, key string) error {
	query := fmt.Sprintf("DELETE FROM %s_locks WHERE key = $1", s.tableName)
	_, err := s.db.ExecContext(ctx, query, key)
	return err
}

// Close closes the database connection
func (s *Storage) Close() error {
	return s.db.Close()
}

// Package gorm provides a GORM-based storage backend for gopotency.
// Using GORM allows the package to be database-agnostic, supporting PostgreSQL,
// MySQL, SQL Server, and SQLite through GORM's dialect system.
package gorm

import (
	"context"
	"encoding/json"
	"time"

	idempotency "github.com/fco-gt/gopotency"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// IdempotencyRecord is the GORM model for storing idempotency data.
type IdempotencyRecord struct {
	Key       string    `gorm:"primaryKey;size:255"`
	Data      []byte    `gorm:"type:blob;not null"` // 'blob' works across most dialects
	ExpiresAt time.Time `gorm:"index;not null"`
}

// IdempotencyLock is the GORM model for distributed locking.
type IdempotencyLock struct {
	Key       string    `gorm:"primaryKey;size:255"`
	ExpiresAt time.Time `gorm:"not null"`
}

// Storage is a GORM implementation of idempotency.Storage.
type Storage struct {
	db *gorm.DB
}

// NewGormStorage creates a new GORM storage instance.
// It is recommended to run db.AutoMigrate(&IdempotencyRecord{}, &IdempotencyLock{}) before use.
func NewGormStorage(db *gorm.DB) *Storage {
	return &Storage{
		db: db,
	}
}

// Get retrieves an idempotency record by key.
func (s *Storage) Get(ctx context.Context, key string) (*idempotency.Record, error) {
	var record IdempotencyRecord
	result := s.db.WithContext(ctx).First(&record, "key = ?", key)

	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, idempotency.NewStorageError("get", result.Error)
	}

	// Check expiration
	if time.Now().After(record.ExpiresAt) {
		_ = s.Delete(ctx, key)
		return nil, nil
	}

	var r idempotency.Record
	if err := json.Unmarshal(record.Data, &r); err != nil {
		return nil, idempotency.NewStorageError("unmarshal", err)
	}

	return &r, nil
}

// Set stores an idempotency record.
func (s *Storage) Set(ctx context.Context, record *idempotency.Record, ttl time.Duration) error {
	data, err := json.Marshal(record)
	if err != nil {
		return idempotency.NewStorageError("marshal", err)
	}

	gormRecord := IdempotencyRecord{
		Key:       record.Key,
		Data:      data,
		ExpiresAt: time.Now().Add(ttl),
	}

	// Cross-database Upsert using GORM clauses
	err = s.db.WithContext(ctx).Clauses(clause.OnConflict{
		UpdateAll: true,
	}).Create(&gormRecord).Error

	if err != nil {
		return idempotency.NewStorageError("set", err)
	}

	return nil
}

// Delete removes an idempotency record and its associated lock.
func (s *Storage) Delete(ctx context.Context, key string) error {
	err := s.db.WithContext(ctx).Delete(&IdempotencyRecord{}, "key = ?", key).Error
	if err != nil {
		return idempotency.NewStorageError("delete", err)
	}
	return s.Unlock(ctx, key)
}

// Exists checks if a record exists and is not expired.
func (s *Storage) Exists(ctx context.Context, key string) (bool, error) {
	var count int64
	err := s.db.WithContext(ctx).Model(&IdempotencyRecord{}).
		Where("key = ? AND expires_at > ?", key, time.Now()).
		Count(&count).Error

	if err != nil {
		return false, idempotency.NewStorageError("exists", err)
	}

	return count > 0, nil
}

// TryLock attempts to acquire a distributed lock.
func (s *Storage) TryLock(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	now := time.Now()
	expiresAt := now.Add(ttl)

	// Clean up expired lock first if it exists (using GORM to be cross-DB)
	s.db.WithContext(ctx).Where("key = ? AND expires_at < ?", key, now).Delete(&IdempotencyLock{})

	// Try to create the lock
	err := s.db.WithContext(ctx).Create(&IdempotencyLock{
		Key:       key,
		ExpiresAt: expiresAt,
	}).Error

	if err != nil {
		// If entry already exists, lock failed
		return false, nil
	}

	return true, nil
}

// Unlock releases a lock.
func (s *Storage) Unlock(ctx context.Context, key string) error {
	return s.db.WithContext(ctx).Delete(&IdempotencyLock{}, "key = ?", key).Error
}

// Close is a no-op for GORM storage as the user manages the DB connection.
func (s *Storage) Close() error {
	return nil
}

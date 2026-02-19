// Package redis provides a Redis storage backend for gopotency.
// This backend is suitable for production environments where multiple
// instances of an application need to share idempotency records.
package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	idempotency "github.com/fco-gt/gopotency"
	"github.com/redis/go-redis/v9"
)

// RedisStorage implements the idempotency.Storage interface using Redis.
// It uses JSON serialization to store the idempotency records and
// Redis distributed locking to handle concurrent requests.
type RedisStorage struct {
	client *redis.Client
}

// NewRedisStorage initializes a new Redis client and checks the connection.
// addr should be in the format "host:port". password can be empty if no auth is required.
func NewRedisStorage(ctx context.Context, addr string, password string) (*RedisStorage, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       0,
	})

	// Verify that the connection is active
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	return &RedisStorage{
		client: client,
	}, nil
}

// Get retrieves an idempotency record from Redis by its key.
// If the key is not found, it returns (nil, nil) instead of an error.
func (s *RedisStorage) Get(ctx context.Context, key string) (*idempotency.Record, error) {
	val, err := s.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // Record doesn't exist in Redis
		}
		return nil, idempotency.NewStorageError("get", err)
	}

	var r idempotency.Record
	if err := json.Unmarshal([]byte(val), &r); err != nil {
		return nil, fmt.Errorf("failed to unmarshal record: %w", err)
	}

	return &r, nil
}

// Set saves an idempotency record in Redis with a specific expiration time (TTL).
// The record is serialized to JSON before being stored.
func (s *RedisStorage) Set(ctx context.Context, record *idempotency.Record, ttl time.Duration) error {
	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("failed to marshal record: %w", err)
	}
	// Use the standard SET command with expiration
	return s.client.Set(ctx, record.Key, data, ttl).Err()
}

// Delete removes an idempotency record from Redis.
func (s *RedisStorage) Delete(ctx context.Context, key string) error {
	return s.client.Del(ctx, key).Err()
}

// Exists checks if an idempotency record exists in Redis for the given key.
func (s *RedisStorage) Exists(ctx context.Context, key string) (bool, error) {
	n, err := s.client.Exists(ctx, key).Result()
	return n > 0, err
}

// TryLock attempts to acquire a distributed lock for the given key.
// It uses "SET key value NX TTL" to ensure only one client can hold the lock.
// This prevents multiple identical requests from being processed simultaneously.
func (s *RedisStorage) TryLock(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	res, err := s.client.SetArgs(ctx, "lock:"+key, "1", redis.SetArgs{
		Mode: "NX", // Only set if the key does NOT exist
		TTL:  ttl,
	}).Result()

	if err != nil {
		if err == redis.Nil {
			return false, nil // Another request already holds the lock
		}
		return false, err
	}
	return res == "OK", nil
}

// Unlock releases the distributed lock for the given key by deleting it.
func (s *RedisStorage) Unlock(ctx context.Context, key string) error {
	return s.client.Del(ctx, "lock:"+key).Err()
}

// Close terminates the Redis client connection.
func (s *RedisStorage) Close() error {
	return s.client.Close()
}

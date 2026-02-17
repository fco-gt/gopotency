// Package hash provides request hashing implementations for idempotency validation.
//
// Request hashers are used to compute a hash of incoming requests to detect
// if two requests with the same idempotency key have different content.
package hash

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/fco-gt/gopotency"
)

// BodyHasher creates a request hasher that hashes the request body
func BodyHasher() idempotency.RequestHasher {
	return &bodyHasher{}
}

type bodyHasher struct{}

func (b *bodyHasher) Hash(req *idempotency.Request) (string, error) {
	if len(req.Body) == 0 {
		return "", nil
	}

	hash := sha256.Sum256(req.Body)
	return hex.EncodeToString(hash[:]), nil
}

// FullHasher creates a request hasher that hashes method + path + body
func FullHasher() idempotency.RequestHasher {
	return &fullHasher{}
}

type fullHasher struct{}

func (f *fullHasher) Hash(req *idempotency.Request) (string, error) {
	data := fmt.Sprintf("%s:%s:%s", req.Method, req.Path, string(req.Body))
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:]), nil
}

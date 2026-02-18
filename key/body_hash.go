package key

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	idempotency "github.com/fco-gt/gopotency"
)

// BodyHash creates a key strategy that generates a key from the request body hash
// The key is computed as: SHA256(method + path + body)
func BodyHash() idempotency.KeyStrategy {
	return &bodyHashGenerator{}
}

type bodyHashGenerator struct{}

func (b *bodyHashGenerator) Generate(req *idempotency.Request) (string, error) {
	// Combine method, path, and body
	data := fmt.Sprintf("%s:%s:%s", req.Method, req.Path, string(req.Body))

	// Compute SHA256 hash
	hash := sha256.Sum256([]byte(data))

	return hex.EncodeToString(hash[:]), nil
}

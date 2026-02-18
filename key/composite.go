package key

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	idempotency "github.com/fco-gt/gopotency"
)

// Composite creates a key strategy that combines header-based key with request hash
// If header is present, uses it; otherwise falls back to body hash
func Composite(headerName string) idempotency.KeyStrategy {
	return &compositeGenerator{
		headerName: headerName,
	}
}

type compositeGenerator struct {
	headerName string
}

func (c *compositeGenerator) Generate(req *idempotency.Request) (string, error) {
	// Try to get key from header first
	if req.Headers != nil {
		values := req.Headers[c.headerName]
		if len(values) > 0 && values[0] != "" {
			return values[0], nil
		}
	}

	// Fall back to body hash
	data := fmt.Sprintf("%s:%s:%s", req.Method, req.Path, string(req.Body))
	hash := sha256.Sum256([]byte(data))

	return hex.EncodeToString(hash[:]), nil
}

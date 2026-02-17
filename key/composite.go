package key

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/fco-gt/idempotency-go"
)

// Composite creates a key generator that combines header-based key with request hash
// If header is present, uses it; otherwise falls back to body hash
func Composite(headerName string) idempotency.KeyGenerator {
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

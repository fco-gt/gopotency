// Package key provides strategies for generating idempotency keys from HTTP requests
package key

import idempotency "github.com/fco-gt/gopotency"

// HeaderBased creates a key strategy that extracts the key from a request header
func HeaderBased(headerName string) idempotency.KeyStrategy {
	return &headerGenerator{
		headerName: headerName,
	}
}

type headerGenerator struct {
	headerName string
}

func (h *headerGenerator) Generate(req *idempotency.Request) (string, error) {
	if req.Headers == nil {
		return "", nil
	}

	values := req.Headers[h.headerName]
	if len(values) == 0 {
		return "", nil
	}

	return values[0], nil
}

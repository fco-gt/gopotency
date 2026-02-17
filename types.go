package idempotency

import "time"

// RecordStatus represents the status of an idempotency record
type RecordStatus string

const (
	// StatusPending indicates a request is currently being processed
	StatusPending RecordStatus = "pending"

	// StatusCompleted indicates a request has been successfully processed
	StatusCompleted RecordStatus = "completed"

	// StatusFailed indicates a request processing failed
	StatusFailed RecordStatus = "failed"
)

// Record represents a stored idempotency record
type Record struct {
	// Key is the idempotency key
	Key string

	// RequestHash is a hash of the request content for validation
	RequestHash string

	// Status is the current status of the request
	Status RecordStatus

	// Response contains the cached response if status is completed
	Response *CachedResponse

	// CreatedAt is when the record was created
	CreatedAt time.Time

	// ExpiresAt is when the record should expire
	ExpiresAt time.Time
}

// CachedResponse represents a cached HTTP response
type CachedResponse struct {
	// StatusCode is the HTTP status code
	StatusCode int

	// Headers are the response headers
	Headers map[string][]string

	// Body is the response body
	Body []byte

	// ContentType is the content type of the response
	ContentType string
}

// Request represents an incoming HTTP request for idempotency checking
type Request struct {
	// Method is the HTTP method (GET, POST, etc.)
	Method string

	// Path is the request path
	Path string

	// Headers are the request headers
	Headers map[string][]string

	// Body is the request body
	Body []byte

	// IdempotencyKey is the extracted or generated idempotency key
	IdempotencyKey string
}

// Response represents an HTTP response to be cached
type Response struct {
	// StatusCode is the HTTP status code
	StatusCode int

	// Headers are the response headers
	Headers map[string][]string

	// Body is the response body
	Body []byte

	// ContentType is the content type of the response
	ContentType string
}

// ToCachedResponse converts a Response to a CachedResponse
func (r *Response) ToCachedResponse() *CachedResponse {
	return &CachedResponse{
		StatusCode:  r.StatusCode,
		Headers:     r.Headers,
		Body:        r.Body,
		ContentType: r.ContentType,
	}
}

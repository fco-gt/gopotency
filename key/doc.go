// Package key provides strategies for generating idempotency keys from HTTP requests.
//
// Available strategies:
//
// HeaderBased: Extracts the key from a request header (e.g., "Idempotency-Key")
//
//	strategy := key.HeaderBased("Idempotency-Key")
//
// BodyHash: Generates a key from the hash of the request content (method + path + body)
//
//	strategy := key.BodyHash()
//
// Composite: Tries header-based first, falls back to body hash if header is not present
//
//	strategy := key.Composite("Idempotency-Key")
package key

# Gin Basic Example

This example demonstrates basic idempotency handling with Gin Gonic using in-memory storage.

## Running the Example

```bash
cd examples/gin-basic
go run main.go
```

## Testing Idempotency

### First Request (New)
```bash
curl -X POST http://localhost:8080/payment \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: payment-001' \
  -d '{"amount":100.50,"currency":"USD"}'
```

Response:
```json
{
  "status": "success",
  "amount": 100.50,
  "currency": "USD",
  "timestamp": 1708123456
}
```

### Second Request (Cached)
Same request with same idempotency key:

```bash
curl -X POST http://localhost:8080/payment \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: payment-001' \
  -d '{"amount":100.50,"currency":"USD"}'
```

Response (with `X-Idempotent-Replayed: true` header):
```json
{
  "status": "success",
  "amount": 100.50,
  "currency": "USD",
  "timestamp": 1708123456
}
```

Note: The timestamp is the same as the first request!

### Request Mismatch (Different Body, Same Key)
```bash
curl -X POST http://localhost:8080/payment \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: payment-001' \
  -d '{"amount":200.00,"currency":"EUR"}'
```

Response:
```json
{
  "error": "request with same idempotency key has different content"
}
```

Status Code: `422 Unprocessable Entity`

## Features Demonstrated

- ✅ Request deduplication
- ✅ Response caching
- ✅ Request validation (body mismatch detection)
- ✅ In-memory storage
- ✅ Multiple endpoints with idempotency

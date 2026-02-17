# HTTP Basic Example

This example demonstrates basic idempotency handling with standard `net/http` using in-memory storage.

## Running the Example

```bash
cd examples/http-basic
go run main.go
```

## Testing Idempotency

### First Request (New)
```bash
curl -X POST http://localhost:8080/payment \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: payment-001' \
  -d '{"amount":250.00,"currency":"EUR"}'
```

Response:
```json
{
  "status": "success",
  "amount": 250.00,
  "currency": "EUR",
  "timestamp": 1708123456
}
```

### Second Request (Cached)
Same request with same idempotency key:

```bash
curl -X POST http://localhost:8080/payment \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: payment-001' \
  -d '{"amount":250.00,"currency":"EUR"}'
```

Response (with `X-Idempotent-Replayed: true` header):
```json
{
  "status": "success",
  "amount": 250.00,
  "currency": "EUR",
  "timestamp": 1708123456
}
```

Note: The timestamp is identical to the first request!

## Features Demonstrated

- ✅ Standard library HTTP middleware
- ✅ Request deduplication
- ✅ Response caching
- ✅ In-memory storage

# GoPotency

[![Go Version](https://img.shields.io/github/go-mod/go-version/fco-gt/gopotency)](https://golang.org/)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

A flexible, framework-agnostic Go package for handling idempotency in HTTP APIs.

## 🎯 Features

- ✅ **Framework Agnostic**: Works with Gin, standard `net/http`, Echo, and more
- ✅ **Multiple Storage Backends**: In-memory, Redis, PostgreSQL support
- ✅ **Flexible Key Strategies**: Header-based or auto-generated from request content
- ✅ **Thread-Safe**: Built for concurrent environments
- ✅ **Configurable**: Extensive options for TTL, lock timeouts, and more
- ✅ **Production Ready**: Comprehensive testing and error handling

## 📦 Installation

```bash
go get github.com/fco-gt/gopotency
```

## 🚀 Quick Start

### With Gin

```go
package main

import (
    "github.com/fco-gt/gopotency"
    "github.com/fco-gt/gopotency/middleware/gin"
    "github.com/fco-gt/gopotency/storage/memory"
    "github.com/gin-gonic/gin"
    "time"
)

func main() {
    // Create storage
    store := memory.NewMemoryStorage()
    
    // Create idempotency manager
    manager, _ := idempotency.NewManager(idempotency.Config{
        Storage: store,
        TTL:     24 * time.Hour,
    })
    
    // Setup Gin router
    router := gin.Default()
    router.Use(ginmw.Idempotency(manager))
    
    router.POST("/payment", func(c *gin.Context) {
        // Your handler logic
        c.JSON(200, gin.H{"status": "processed"})
    })
    
    router.Run(":8080")
}
```

### With Standard HTTP

```go
package main

import (
    "github.com/fco-gt/gopotency"
    "github.com/fco-gt/gopotency/middleware/http"
    "github.com/fco-gt/gopotency/storage/memory"
    "net/http"
    "time"
)

func main() {
    // Create storage
    store := memory.NewMemoryStorage()
    
    // Create idempotency manager
    manager, _ := idempotency.NewManager(idempotency.Config{
        Storage: store,
        TTL:     24 * time.Hour,
    })
    
    // Wrap your handler
    mux := http.NewServeMux()
    handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte(`{"status": "processed"}`))
    })
    
    mux.Handle("/payment", httpmw.Idempotency(manager)(handler))
    
    http.ListenAndServe(":8080", mux)
}
```

## 📖 Documentation

### Configuration Options

```go
type Config struct {
    // Storage backend (required)
    Storage Storage
    
    // TTL for idempotency records (default: 24h)
    TTL time.Duration
    
    // Lock timeout to prevent deadlocks (default: 5m)
    LockTimeout time.Duration
    
    // Key strategy: how to generate idempotency keys
    // Default: HeaderBased("Idempotency-Key")
    KeyStrategy KeyStrategy
    
    // Only apply to specific HTTP methods
    // Default: ["POST", "PUT", "PATCH", "DELETE"]
    AllowedMethods []string
    
    // Custom error handler (optional)
    ErrorHandler func(error) (int, interface{})
}
```

### Storage Backends

#### In-Memory (Development/Testing)

```go
import "github.com/fco-gt/gopotency/storage/memory"

store := memory.NewMemoryStorage()
```

#### Redis (Production)

```go
import (
    "github.com/fco-gt/gopotency/storage/redis"
    "github.com/redis/go-redis/v9"
)

client := redis.NewClient(&redis.Options{
    Addr: "localhost:6379",
})

store := redis.NewRedisStorage(client, redis.Options{
    KeyPrefix:  "idempotency:",
    DefaultTTL: 24 * time.Hour,
})
```

### Key Strategies

#### Header-Based (Default)

Client sends `Idempotency-Key` header:

```go
import "github.com/fco-gt/gopotency/key"

config := idempotency.Config{
    Storage:     store,
    KeyStrategy: key.HeaderBased("Idempotency-Key"),
}
```

#### Auto-Hash (Automatic)

Generates key from request content (method + path + body):

```go
import "github.com/fco-gt/gopotency/key"

config := idempotency.Config{
    Storage:     store,
    KeyStrategy: key.BodyHash(),
}
```

## 🔧 Advanced Usage

### Custom Error Handler

```go
config := idempotency.Config{
    Storage: store,
    ErrorHandler: func(err error) (int, interface{}) {
        return 500, map[string]string{
            "error": err.Error(),
        }
    },
}
```

### Selective Application

```go
// Only apply to specific methods
config := idempotency.Config{
    Storage:        store,
    AllowedMethods: []string{"POST", "DELETE"},
}
```

## 📊 How It Works

1.  **Request arrives** with `Idempotency-Key` header
2.  **Check storage** for existing record
3.  **Three scenarios**:
    -   ✅ **Completed**: Return cached response immediately
    -   ⏳ **Pending**: Return 409 Conflict (request already in progress)
    -   🆕 **New**: Acquire lock, process request, store response

## 🤝 Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## 📄 License

MIT License - see LICENSE file for details

## 🙏 Acknowledgments

Inspired by idempotency implementations from Stripe, PayPal, and other leading APIs.

## 📚 Examples

Check the [examples](./examples) directory for more use cases:

- [Gin Basic](./examples/gin-basic) - Simple Gin integration
- [HTTP Basic](./examples/http-basic) - Standard library usage
- [Custom Storage](./examples/custom-storage) - Implementing custom storage backend

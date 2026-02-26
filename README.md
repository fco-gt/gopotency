# GoPotency

[![Go Version](https://img.shields.io/github/go-mod/go-version/fco-gt/gopotency)](https://golang.org/)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Go Tests](https://github.com/fco-gt/gopotency/actions/workflows/go-tests.yml/badge.svg)](https://github.com/fco-gt/gopotency/actions/workflows/go-tests.yml)

A flexible, framework-agnostic Go package for handling idempotency in HTTP APIs.

## 🎯 Features

- ✅ **Framework Agnostic**: Works with Gin, standard `net/http`, Echo, and more.
- ✅ **Multiple Storage Backends**: In-memory, Redis, SQL, and **GORM** support.
- ✅ **Database Agnostic**: Use any DB with GORM (PostgreSQL, MySQL, SQL Server, SQLite).
- ✅ **Distributed Locking**: Built-in support for multiple instances.
- ✅ **Production Ready**: Comprehensive testing, benchmarks, and CI/CD.

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
    ginmw "github.com/fco-gt/gopotency/middleware/gin"
    "github.com/fco-gt/gopotency/storage/memory"
    "github.com/gin-gonic/gin"
    "time"
)

func main() {
    store := memory.NewMemoryStorage()
    manager, _ := idempotency.NewManager(idempotency.Config{
        Storage: store,
        TTL:     24 * time.Hour,
    })

    r := gin.Default()
    r.Use(ginmw.Idempotency(manager))

    r.POST("/orders", func(c *gin.Context) {
        c.JSON(201, gin.H{"order_id": "ORD-123", "status": "created"})
    })

    r.Run(":8080")
}
```

### With Standard HTTP

```go
package main

import (
    "github.com/fco-gt/gopotency"
    httpmw "github.com/fco-gt/gopotency/middleware/http"
    "github.com/fco-gt/gopotency/storage/memory"
    "net/http"
    "time"
)

func main() {
    store := memory.NewMemoryStorage()
    manager, _ := idempotency.NewManager(idempotency.Config{
        Storage: store,
        TTL:     24 * time.Hour,
    })

    mux := http.NewServeMux()
    handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte(`{"status": "processed"}`))
    })

    mux.Handle("/process", httpmw.Idempotency(manager)(handler))
    http.ListenAndServe(":8080", mux)
}
```

## 📖 Documentation

### Configuration Options

```go
type Config struct {
    Storage        Storage       // Required: Memory, Redis, SQL, or GORM
    TTL            time.Duration // Default: 24h
    LockTimeout    time.Duration // Default: 5m
    KeyStrategy    KeyStrategy   // Default: HeaderBased("Idempotency-Key")
    AllowedMethods []string      // Default: ["POST", "PUT", "PATCH", "DELETE"]
    RequireKey     bool          // If true, returns 400 if key is missing (Default: false)
    ErrorHandler   func(error) (int, any)
}
```

### Route-Specific Middleware

GoPotency allows you to be granular. If you provide an `Idempotency-Key` in the request, the middleware will process it regardless of the method.

For critical routes, you can enable `RequireKey: true` to ensure no one accidentally skips idempotency.

### Storage Backends

#### In-Memory (Dev/Single Instance)

```go
import "github.com/fco-gt/gopotency/storage/memory"
store := memory.NewMemoryStorage()
```

#### Redis (Distributed)

```go
import "github.com/fco-gt/gopotency/storage/redis"
store, err := redis.NewRedisStorage(ctx, "localhost:6379", "password")
```

#### GORM (Database Agnostic)

```go
import (
    idempotencyGorm "github.com/fco-gt/gopotency/storage/gorm"
    "gorm.io/gorm"
)
store := idempotencyGorm.NewGormStorage(db)
```

#### SQL (Postgres/SQLite)

```go
import idempotencySQL "github.com/fco-gt/gopotency/storage/sql"
store := idempotencySQL.NewSQLStorage(db, "idempotency_records")
```

## �️ Development

We use a `Makefile` to streamline development:

```bash
make test    # Run all tests
make bench   # Run performance benchmarks
make build   # Build all examples
```

## 📊 Benchmarks

GoPotency is optimized for high-performance APIs.

| Operation                  | Time        |
| :------------------------- | :---------- |
| **Idempotency Check**      | ~520 ns/op  |
| **Full Flow (Lock/Store)** | ~1500 ns/op |

## 🤝 Contributing

Contributions are what make the open source community such an amazing place to learn, inspire, and create. Any contributions you make are **greatly appreciated**.

1. Fork the Project
2. Create your Feature Branch (`git checkout -b feature/AmazingFeature`)
3. Commit your Changes (`git commit -m 'Add some AmazingFeature'`)
4. Push to the Branch (`git push origin feature/AmazingFeature`)
5. Open a Pull Request

## 📄 License

Distributed under the MIT License. See `LICENSE` for more information.

## 📚 Examples

- [Gin Basic](./examples/gin-basic)
- [HTTP Basic](./examples/http-basic)
- [Redis Basic](./examples/redis-basic)
- [SQL Basic](./examples/sql-basic)
- [GORM Basic](./examples/gorm-basic)

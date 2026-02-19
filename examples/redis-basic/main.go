package main

import (
	"context"
	"log"
	"net/http"
	"time"

	idempotency "github.com/fco-gt/gopotency"
	ginmw "github.com/fco-gt/gopotency/middleware/gin"
	idempotencyRedis "github.com/fco-gt/gopotency/storage/redis"
	"github.com/gin-gonic/gin"
)

func main() {
	// 1. Initialize Redis Storage
	// For this example, we'll assume a local Redis instance.
	// In production, use environment variables for connection details.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	store, err := idempotencyRedis.NewRedisStorage(ctx, "localhost:6379", "")
	if err != nil {
		log.Printf("Warning: Could not connect to Redis: %v", err)
		log.Printf("Make sure Redis is running on localhost:6379")
		// In a real app, you might want to switch to memory storage as fallback
		// or fatal out depending on your requirements.
		return
	}
	defer store.Close()

	// 2. Create Idempotency Manager
	manager, err := idempotency.NewManager(idempotency.Config{
		Storage: store,
		TTL:     24 * time.Hour,
	})
	if err != nil {
		log.Fatalf("Failed to create manager: %v", err)
	}

	// 3. Setup Gin Router
	r := gin.Default()

	// 4. Use Idempotency Middleware
	r.Use(ginmw.Idempotency(manager))

	// 5. Protected Endpoint
	r.POST("/orders", func(c *gin.Context) {
		// Simulate some work
		time.Sleep(500 * time.Millisecond)

		c.JSON(http.StatusCreated, gin.H{
			"order_id": "ORD-12345",
			"status":   "confirmed",
			"message":  "Order processed successfully using Redis storage",
		})
	})

	log.Println("Redis example starting on :8084")
	r.Run(":8084")
}

package main

import (
	"log"
	"time"

	idempotency "github.com/fco-gt/gopotency"
	"github.com/fco-gt/gopotency/key"
	ginmw "github.com/fco-gt/gopotency/middleware/gin"
	"github.com/fco-gt/gopotency/storage/memory"
	"github.com/gin-gonic/gin"
)

func main() {
	// Create in-memory storage
	store := memory.NewMemoryStorage()
	defer store.Close()

	// Create idempotency manager
	manager, err := idempotency.NewManager(idempotency.Config{
		Storage:     store,
		TTL:         24 * time.Hour,
		KeyStrategy: key.BodyHash(),
	})
	if err != nil {
		log.Fatal(err)
	}
	defer manager.Close()

	// Create Gin router
	router := gin.Default()

	// Apply idempotency middleware
	router.Use(ginmw.Idempotency(manager))

	// Define routes
	router.POST("/payment", func(c *gin.Context) {
		var payload struct {
			Amount   float64 `json:"amount"`
			Currency string  `json:"currency"`
		}

		if err := c.BindJSON(&payload); err != nil {
			c.JSON(400, gin.H{"error": "invalid payload"})
			return
		}

		// Simulate payment processing
		log.Printf("Processing payment: %.2f %s", payload.Amount, payload.Currency)
		time.Sleep(100 * time.Millisecond)

		c.JSON(200, gin.H{
			"status":    "success",
			"amount":    payload.Amount,
			"currency":  payload.Currency,
			"timestamp": time.Now().Unix(),
		})
	})

	router.POST("/order", func(c *gin.Context) {
		var payload struct {
			ProductID string `json:"product_id"`
			Quantity  int    `json:"quantity"`
		}

		if err := c.BindJSON(&payload); err != nil {
			c.JSON(400, gin.H{"error": "invalid payload"})
			return
		}

		// Simulate order creation
		log.Printf("Creating order: %s x%d", payload.ProductID, payload.Quantity)
		time.Sleep(50 * time.Millisecond)

		c.JSON(201, gin.H{
			"status":     "created",
			"order_id":   "ORD-12345",
			"product_id": payload.ProductID,
			"quantity":   payload.Quantity,
		})
	})

	// Health check (no idempotency - GET method)
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "healthy"})
	})

	log.Println("Server starting on :8080")
	log.Println("Try these commands:")
	log.Println("  curl -X POST http://localhost:8080/payment -H 'Content-Type: application/json' -H 'Idempotency-Key: key123' -d '{\"amount\":100.50,\"currency\":\"USD\"}'")
	log.Println("  curl -X POST http://localhost:8080/payment -H 'Content-Type: application/json' -H 'Idempotency-Key: key123' -d '{\"amount\":100.50,\"currency\":\"USD\"}' # Same key, will return cached response")

	if err := router.Run(":8080"); err != nil {
		log.Fatal(err)
	}
}

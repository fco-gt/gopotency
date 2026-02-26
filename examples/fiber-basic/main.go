package main

import (
	idempotency "github.com/fco-gt/gopotency"
	fibermw "github.com/fco-gt/gopotency/middleware/fiber"
	"github.com/fco-gt/gopotency/storage/memory"
	"github.com/gofiber/fiber/v2"
)

func main() {
	app := fiber.New()

	// Initialize idempotency manager with in-memory storage
	store := memory.NewMemoryStorage()
	manager, _ := idempotency.NewManager(idempotency.Config{
		Storage: store,
	})

	// Register idempotency middleware
	app.Use(fibermw.Idempotency(manager))

	// Example idempotent route
	app.Post("/payments", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":         "Payment received",
			"transaction_id": "tx_98765",
		})
	})

	app.Listen(":8080")
}

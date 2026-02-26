package main

import (
	"net/http"

	idempotency "github.com/fco-gt/gopotency"
	echomw "github.com/fco-gt/gopotency/middleware/echo"
	"github.com/fco-gt/gopotency/storage/memory"
	"github.com/labstack/echo/v4"
)

func main() {
	e := echo.New()

	// Initialize idempotency manager with in-memory storage
	store := memory.NewMemoryStorage()
	manager, _ := idempotency.NewManager(idempotency.Config{
		Storage: store,
	})

	// Register idempotency middleware
	e.Use(echomw.Idempotency(manager))

	// Example idempotent route
	e.POST("/orders", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{
			"message":  "Order processed successfully",
			"order_id": "12345",
		})
	})

	e.Logger.Fatal(e.Start(":8080"))
}

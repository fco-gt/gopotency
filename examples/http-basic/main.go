package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/fco-gt/gopotency"
	httpmw "github.com/fco-gt/gopotency/middleware/http"
	"github.com/fco-gt/gopotency/storage/memory"
)

type PaymentRequest struct {
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
}

type PaymentResponse struct {
	Status    string  `json:"status"`
	Amount    float64 `json:"amount"`
	Currency  string  `json:"currency"`
	Timestamp int64   `json:"timestamp"`
}

func main() {
	// Create in-memory storage
	store := memory.NewMemoryStorage()
	defer store.Close()

	// Create idempotency manager
	manager, err := idempotency.NewManager(idempotency.Config{
		Storage: store,
		TTL:     24 * time.Hour,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer manager.Close()

	// Create HTTP server
	mux := http.NewServeMux()

	// Payment endpoint with idempotency
	paymentHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req PaymentRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"invalid payload"}`, http.StatusBadRequest)
			return
		}

		// Simulate payment processing
		log.Printf("Processing payment: %.2f %s", req.Amount, req.Currency)
		time.Sleep(100 * time.Millisecond)

		resp := PaymentResponse{
			Status:    "success",
			Amount:    req.Amount,
			Currency:  req.Currency,
			Timestamp: time.Now().Unix(),
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	})

	// Apply idempotency middleware
	mux.Handle("/payment", httpmw.Idempotency(manager)(paymentHandler))

	// Health check (no idempotency)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy"}`))
	})

	log.Println("Server starting on :8080")
	log.Println("Try these commands:")
	log.Println("  curl -X POST http://localhost:8080/payment -H 'Content-Type: application/json' -H 'Idempotency-Key: key456' -d '{\"amount\":250.00,\"currency\":\"EUR\"}'")
	log.Println("  curl -X POST http://localhost:8080/payment -H 'Content-Type: application/json' -H 'Idempotency-Key: key456' -d '{\"amount\":250.00,\"currency\":\"EUR\"}' # Same key, will return cached response")

	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatal(err)
	}
}

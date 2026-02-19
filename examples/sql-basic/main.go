package main

import (
	"database/sql"
	"log"
	"net/http"
	"time"

	idempotency "github.com/fco-gt/gopotency"
	httpmw "github.com/fco-gt/gopotency/middleware/http"
	idempotencySQL "github.com/fco-gt/gopotency/storage/sql"
	_ "github.com/mattn/go-sqlite3" // Import your driver
)

func main() {
	// 1. Initialize DB (using SQLite for this example)
	db, err := sql.Open("sqlite3", "./idempotency.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// 2. Create tables if they don't exist
	// In production, use a migration tool!
	_, _ = db.Exec(`CREATE TABLE IF NOT EXISTS idempotency_records (
		key TEXT PRIMARY KEY,
		data BLOB NOT NULL,
		expires_at DATETIME NOT NULL
	)`)
	_, _ = db.Exec(`CREATE TABLE IF NOT EXISTS idempotency_records_locks (
		key TEXT PRIMARY KEY,
		expires_at DATETIME NOT NULL
	)`)

	// 3. Create storage
	store := idempotencySQL.NewSQLStorage(db, "idempotency_records")

	// 4. Create manager
	manager, err := idempotency.NewManager(idempotency.Config{
		Storage: store,
		TTL:     24 * time.Hour,
	})
	if err != nil {
		log.Fatal(err)
	}

	// 5. Setup Router
	mux := http.NewServeMux()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"success","message":"processed with SQL storage"}`))
	})

	// Wrap with middleware
	mux.Handle("/submit", httpmw.Idempotency(manager)(handler))

	log.Println("SQL Example running on :8082")
	http.ListenAndServe(":8082", mux)
}

package main

import (
	"log"
	"net/http"
	"time"

	idempotency "github.com/fco-gt/gopotency"
	httpmw "github.com/fco-gt/gopotency/middleware/http"
	idempotencyGorm "github.com/fco-gt/gopotency/storage/gorm"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func main() {
	// 1. Initialize GORM with SQLite
	db, err := gorm.Open(sqlite.Open("gopotency.db"), &gorm.Config{})
	if err != nil {
		log.Fatal(err)
	}

	// 2. Automigrate the idempotency models
	err = db.AutoMigrate(&idempotencyGorm.IdempotencyRecord{}, &idempotencyGorm.IdempotencyLock{})
	if err != nil {
		log.Fatal(err)
	}

	// 3. Create GORM storage
	store := idempotencyGorm.NewGormStorage(db)

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
		w.Write([]byte(`{"status":"success","message":"processed using GORM storage"}`))
	})

	// Wrap with middleware
	mux.Handle("/submit", httpmw.Idempotency(manager)(handler))

	log.Println("GORM Example running on :8083")
	http.ListenAndServe(":8083", mux)
}

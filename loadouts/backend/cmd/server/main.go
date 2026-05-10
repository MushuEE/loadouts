package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/gmccloskey/loadouts/backend/api/handlers"
	"github.com/gmccloskey/loadouts/backend/internal/db"
	"github.com/gmccloskey/loadouts/backend/internal/service"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	connStr := os.Getenv("DATABASE_URL")

	var store db.Store
	var err error

	if connStr == "" {
		log.Println("No DATABASE_URL provided, using In-Memory Store")
		store = db.NewMemoryStore()
	} else {
		log.Printf("Connecting to Postgres: %s", connStr)
		store, err = db.NewPostgresStore(connStr)
		if err != nil {
			log.Fatalf("failed to setup postgres store: %v", err)
		}
	}

	// 2. Setup Service
	invSvc := service.NewInventoryService(store)
	if err := invSvc.Init(context.Background()); err != nil {
		log.Printf("Warning: failed to populate bloom filter: %v", err)
	}

	// 3. Setup Handlers
	itemHandler := handlers.NewItemHandler(invSvc)
	schemaHandler := handlers.NewSchemaHandler(invSvc)

	r := chi.NewRouter()

	// A good base middleware stack
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Routes
	r.Route("/api/v1", func(r chi.Router) {
		r.Mount("/items", itemHandler.Routes())
		r.Mount("/schemas", schemaHandler.Routes())
	})

	log.Println("Starting server on :8080")
	if err := http.ListenAndServe(":8080", r); err != nil {
		log.Fatal(err)
	}
}

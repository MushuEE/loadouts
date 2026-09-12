package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/gmccloskey/loadouts/backend/api/handlers"
	"github.com/gmccloskey/loadouts/backend/internal/auth"
	"github.com/gmccloskey/loadouts/backend/internal/db"
	"github.com/gmccloskey/loadouts/backend/internal/seed"
	"github.com/gmccloskey/loadouts/backend/internal/service"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	connStr := os.Getenv("DATABASE_URL")

	var store db.Store
	var err error
	usingMemory := connStr == ""

	if usingMemory {
		log.Println("No DATABASE_URL provided, using In-Memory Store")
		store = db.NewMemoryStore()
	} else {
		log.Printf("Connecting to Postgres: %s", connStr)
		store, err = db.NewPostgresStore(connStr)
		if err != nil {
			log.Fatalf("failed to setup postgres store: %v", err)
		}
	}

	// 2. Setup Services
	invSvc := service.NewInventoryService(store)
	if err := invSvc.Init(context.Background()); err != nil {
		log.Printf("Warning: failed to populate bloom filter: %v", err)
	}
	identitySvc := service.NewIdentityService(store)
	communitySvc := service.NewCommunityService(store)
	templateSvc := service.NewTemplateService(store, communitySvc)
	loadoutSvc := service.NewLoadoutService(store, invSvc, templateSvc, communitySvc)
	favoriteSvc := service.NewFavoriteService(store, loadoutSvc, communitySvc)
	// A nil fetcher gives the default SSRF-guarded HTTP fetcher; tests inject a fake.
	importSvc := service.NewImportService(store, invSvc, nil)
	// Plugin embed frames are served from SANDBOX_BASE and may only be framed by
	// APP_ORIGIN. Both default to the local dev setup.
	pluginSvc := service.NewPluginService(store, communitySvc, loadoutSvc, invSvc, service.PluginConfig{
		SandboxBase: envOr("SANDBOX_BASE", "http://localhost:8080/sandbox"),
		AppOrigin:   envOr("APP_ORIGIN", "http://localhost:5173"),
	})

	// 3. Seed demo data. Defaults on for the in-memory store (nothing to lose), opt-in
	//    for Postgres via SEED=1.
	if shouldSeed(usingMemory) {
		if err := seed.Run(context.Background(), seed.Services{
			Store:     store,
			Identity:  identitySvc,
			Community: communitySvc,
			Templates: templateSvc,
			Loadouts:  loadoutSvc,
			Inventory: invSvc,
			Plugins:   pluginSvc,
		}); err != nil {
			log.Printf("Warning: seeding failed: %v", err)
		} else {
			log.Println("Seeded demo dataset (profiles: @gearhead, @fitcheck, @trailsponsor)")
		}
	}

	// 4. Setup Handlers
	itemHandler := handlers.NewItemHandler(invSvc, communitySvc)
	schemaHandler := handlers.NewSchemaHandler(invSvc)
	identityHandler := handlers.NewIdentityHandler(identitySvc, loadoutSvc, communitySvc)
	communityHandler := handlers.NewCommunityHandler(communitySvc, templateSvc, loadoutSvc)
	templateHandler := handlers.NewTemplateHandler(templateSvc)
	favoriteHandler := handlers.NewFavoriteHandler(favoriteSvc)
	loadoutHandler := handlers.NewLoadoutHandler(loadoutSvc, favoriteHandler)
	importHandler := handlers.NewImportHandler(importSvc)
	pluginHandler := handlers.NewPluginHandler(pluginSvc)
	sandboxHandler := handlers.NewSandboxHandler(pluginSvc)

	r := chi.NewRouter()

	// A good base middleware stack
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(corsMiddleware)
	// Day 0 auth: resolves the acting profile from the X-Profile-ID header.
	r.Use(auth.Middleware(identitySvc))

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	// Routes
	r.Route("/api/v1", func(r chi.Router) {
		r.Mount("/items", itemHandler.Routes())
		r.Mount("/imports", importHandler.Routes())
		r.Mount("/schemas", schemaHandler.Routes())
		r.Mount("/users", identityHandler.UserRoutes())
		r.Mount("/profiles", identityHandler.ProfileRoutes())
		r.Mount("/communities", communityHandler.Routes())
		r.Mount("/templates", templateHandler.Routes())
		r.Mount("/loadouts", loadoutHandler.Routes())
		r.Mount("/discover", loadoutHandler.DiscoverRoutes())
		r.Mount("/favorites", favoriteHandler.ScopeRoutes())
		r.Mount("/plugins", pluginHandler.Routes())
	})

	// The sandbox lives outside /api/v1 because it serves HTML documents, not JSON, and
	// is the one route that executes code somebody else wrote.
	r.Mount("/sandbox", sandboxHandler.Routes())

	log.Println("Starting server on :8080")
	if err := http.ListenAndServe(":8080", r); err != nil {
		log.Fatal(err)
	}
}

// envOr reads an environment variable with a fallback.
func envOr(name, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(name)); v != "" {
		return v
	}
	return fallback
}

// shouldSeed defaults to true for the in-memory store, and honours SEED=0/1 either way.
func shouldSeed(usingMemory bool) bool {
	switch strings.ToLower(os.Getenv("SEED")) {
	case "1", "true", "yes":
		return true
	case "0", "false", "no":
		return false
	default:
		return usingMemory
	}
}

// corsMiddleware keeps the Vite dev server (5173) able to talk to the API (8080) without
// pulling in another dependency.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" {
			origin = "*"
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Vary", "Origin")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Profile-ID, X-User-ID")
		w.Header().Set("Access-Control-Max-Age", "300")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

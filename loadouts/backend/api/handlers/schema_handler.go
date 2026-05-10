package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/gmccloskey/loadouts/backend/internal/core"
	"github.com/gmccloskey/loadouts/backend/internal/service"
	"github.com/go-chi/chi/v5"
)

type SchemaHandler struct {
	svc *service.InventoryService
}

func NewSchemaHandler(svc *service.InventoryService) *SchemaHandler {
	return &SchemaHandler{svc: svc}
}

func (h *SchemaHandler) Routes() chi.Router {
	r := chi.NewRouter()

	r.Get("/", h.List)
	r.Post("/", h.Register)
	r.Get("/{schemaID}", h.Get)

	return r
}

func (h *SchemaHandler) List(w http.ResponseWriter, r *http.Request) {
	schemas, err := h.svc.ListSchemas(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(schemas)
}

func (h *SchemaHandler) Register(w http.ResponseWriter, r *http.Request) {
	var schema core.SchemaDefinition
	if err := json.NewDecoder(r.Body).Decode(&schema); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := h.svc.RegisterSchema(r.Context(), schema); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(schema)
}

func (h *SchemaHandler) Get(w http.ResponseWriter, r *http.Request) {
	schemaID := chi.URLParam(r, "schemaID")
	version := r.URL.Query().Get("v")
	if version == "" {
		version = "v1" // Default to v1
	}

	schema, err := h.svc.GetSchema(r.Context(), schemaID, version)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(schema)
}

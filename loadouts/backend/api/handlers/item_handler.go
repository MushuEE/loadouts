package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/gmccloskey/loadouts/backend/internal/core"
	"github.com/gmccloskey/loadouts/backend/internal/service"
	"github.com/go-chi/chi/v5"
)

type ItemHandler struct {
	svc *service.InventoryService
}

func NewItemHandler(svc *service.InventoryService) *ItemHandler {
	return &ItemHandler{svc: svc}
}

func (h *ItemHandler) Routes() chi.Router {
	r := chi.NewRouter()

	r.Get("/", h.List)
	r.Post("/", h.Create)
	r.Route("/{itemID}", func(r chi.Router) {
		r.Get("/", h.Get)
		r.Post("/metadata", h.UpdateMetadata)
	})

	return r
}

func (h *ItemHandler) List(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	items, err := h.svc.SearchItems(r.Context(), query)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(items)
}

func (h *ItemHandler) Create(w http.ResponseWriter, r *http.Request) {
	var item core.Item
	if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := h.svc.CreateItem(r.Context(), item); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(item)
}

func (h *ItemHandler) Get(w http.ResponseWriter, r *http.Request) {
	itemID := chi.URLParam(r, "itemID")
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		userID = "anonymous"
	}

	item, err := h.svc.GetMergedItem(r.Context(), itemID, userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(item)
}

func (h *ItemHandler) UpdateMetadata(w http.ResponseWriter, r *http.Request) {
	itemID := chi.URLParam(r, "itemID")
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		http.Error(w, "X-User-ID header required", http.StatusUnauthorized)
		return
	}

	var req struct {
		Overrides core.Metadata `json:"overrides"`
		OpenData  core.Metadata `json:"open_data"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.svc.UpdateMetadata(r.Context(), userID, itemID, req.Overrides, req.OpenData); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

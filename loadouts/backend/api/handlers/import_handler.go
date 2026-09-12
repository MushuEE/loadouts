package handlers

import (
	"net/http"

	"github.com/gmccloskey/loadouts/backend/internal/auth"
	"github.com/gmccloskey/loadouts/backend/internal/service"
	"github.com/go-chi/chi/v5"
)

// ImportHandler exposes the paste-a-URL item import flow.
//
// Preview and commit are separate endpoints because preview is a read-only, potentially
// slow network call and commit is a fast write of already-confirmed data. Splitting them
// also means a failed scrape never leaves a half-created item behind.
type ImportHandler struct {
	svc *service.ImportService
}

func NewImportHandler(svc *service.ImportService) *ImportHandler {
	return &ImportHandler{svc: svc}
}

func (h *ImportHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/suppliers", h.Suppliers)
	r.Post("/preview", h.Preview)
	r.Post("/commit", h.Commit)
	return r
}

// Suppliers lists the retailers we have URL rules for.
func (h *ImportHandler) Suppliers(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"suppliers": h.svc.SupportedSuppliers(),
		"note":      "Links from other stores still import; they just aren't recognized automatically.",
	})
}

// Preview inspects a product URL. It writes nothing, so it is safe to call on every
// paste/keystroke-debounce from the client.
func (h *ImportHandler) Preview(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL string `json:"url"`
	}
	if err := decode(r, &req); err != nil {
		writeError(w, err)
		return
	}

	preview, err := h.svc.Preview(r.Context(), req.URL)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, preview)
}

// Commit creates the catalog item from the draft the user confirmed.
func (h *ImportHandler) Commit(w http.ResponseWriter, r *http.Request) {
	var req service.CommitRequest
	if err := decode(r, &req); err != nil {
		writeError(w, err)
		return
	}

	result, err := h.svc.Commit(r.Context(), req, auth.ProfileID(r.Context()))
	if err != nil {
		writeError(w, err)
		return
	}

	// 200 rather than 201 when the URL was already in the catalog, so a client can tell
	// "we made this for you" from "this already existed".
	status := http.StatusOK
	if result.Created {
		status = http.StatusCreated
	}
	writeJSON(w, status, result)
}

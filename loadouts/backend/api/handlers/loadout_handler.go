package handlers

import (
	"net/http"

	"github.com/gmccloskey/loadouts/backend/internal/auth"
	"github.com/gmccloskey/loadouts/backend/internal/core"
	"github.com/gmccloskey/loadouts/backend/internal/service"
	"github.com/go-chi/chi/v5"
)

// LoadoutHandler exposes the core shareable entity: create, edit, publish, fork, discover.
type LoadoutHandler struct {
	loadouts *service.LoadoutService
}

func NewLoadoutHandler(loadouts *service.LoadoutService) *LoadoutHandler {
	return &LoadoutHandler{loadouts: loadouts}
}

func (h *LoadoutHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.List)
	r.Post("/", h.Create)
	r.Route("/{loadoutID}", func(r chi.Router) {
		r.Get("/", h.Get)
		r.Patch("/", h.Update)
		r.Delete("/", h.Delete)
		r.Put("/entries", h.ReplaceEntries)
		r.Post("/publish", h.Publish)
		r.Post("/fork", h.Fork)
	})
	return r
}

// DiscoverRoutes mounts the public feed at /discover.
func (h *LoadoutHandler) DiscoverRoutes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.Discover)
	return r
}

// List returns the acting profile's own loadouts (drafts included).
func (h *LoadoutHandler) List(w http.ResponseWriter, r *http.Request) {
	profileID := auth.ProfileID(r.Context())
	query := core.DiscoverQuery{
		ProfileID:   defaultQuery(r.URL.Query().Get("profile_id"), profileID),
		CommunityID: r.URL.Query().Get("community_id"),
		TemplateID:  r.URL.Query().Get("template_id"),
		Text:        r.URL.Query().Get("q"),
		Limit:       queryInt(r, "limit", 50),
	}
	summaries, err := h.loadouts.Discover(r.Context(), query, profileID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, summaries)
}

// Discover is the public feed: published, publicly visible loadouts only.
func (h *LoadoutHandler) Discover(w http.ResponseWriter, r *http.Request) {
	query := core.DiscoverQuery{
		Text:        r.URL.Query().Get("q"),
		CommunityID: r.URL.Query().Get("community_id"),
		TemplateID:  r.URL.Query().Get("template_id"),
		OnlyPublic:  true,
		Limit:       queryInt(r, "limit", 50),
	}
	summaries, err := h.loadouts.Discover(r.Context(), query, auth.ProfileID(r.Context()))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, summaries)
}

func (h *LoadoutHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req service.CreateLoadoutRequest
	if err := decode(r, &req); err != nil {
		writeError(w, err)
		return
	}
	detail, err := h.loadouts.Create(r.Context(), auth.ProfileID(r.Context()), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, detail)
}

func (h *LoadoutHandler) Get(w http.ResponseWriter, r *http.Request) {
	detail, err := h.loadouts.Detail(r.Context(), chi.URLParam(r, "loadoutID"), auth.ProfileID(r.Context()))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (h *LoadoutHandler) Update(w http.ResponseWriter, r *http.Request) {
	var req service.UpdateLoadoutRequest
	if err := decode(r, &req); err != nil {
		writeError(w, err)
		return
	}
	detail, err := h.loadouts.Update(r.Context(), auth.ProfileID(r.Context()), chi.URLParam(r, "loadoutID"), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

// ReplaceEntries takes the whole entry tree; the editor owns the full client-side state.
func (h *LoadoutHandler) ReplaceEntries(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Entries []core.LoadoutEntry `json:"entries"`
	}
	if err := decode(r, &req); err != nil {
		writeError(w, err)
		return
	}
	detail, err := h.loadouts.ReplaceEntries(r.Context(), auth.ProfileID(r.Context()), chi.URLParam(r, "loadoutID"), req.Entries)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (h *LoadoutHandler) Publish(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Visibility  core.Visibility `json:"visibility"`
		CommunityID string          `json:"community_id"`
	}
	// A bodyless publish is valid and means "make it public as-is".
	_ = decode(r, &req)

	detail, err := h.loadouts.Publish(r.Context(), auth.ProfileID(r.Context()), chi.URLParam(r, "loadoutID"), req.Visibility, req.CommunityID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (h *LoadoutHandler) Fork(w http.ResponseWriter, r *http.Request) {
	detail, err := h.loadouts.Fork(r.Context(), auth.ProfileID(r.Context()), chi.URLParam(r, "loadoutID"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, detail)
}

func (h *LoadoutHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.loadouts.Delete(r.Context(), auth.ProfileID(r.Context()), chi.URLParam(r, "loadoutID")); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}

func defaultQuery(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

package handlers

import (
	"net/http"

	"github.com/gmccloskey/loadouts/backend/internal/auth"
	"github.com/gmccloskey/loadouts/backend/internal/core"
	"github.com/gmccloskey/loadouts/backend/internal/service"
	"github.com/go-chi/chi/v5"
)

// FavoriteHandler exposes endorsement: a community vouching for a loadout, or a profile
// bookmarking one.
type FavoriteHandler struct {
	favorites *service.FavoriteService
}

func NewFavoriteHandler(favorites *service.FavoriteService) *FavoriteHandler {
	return &FavoriteHandler{favorites: favorites}
}

// LoadoutRoutes mounts under /loadouts/{loadoutID}.
func (h *FavoriteHandler) LoadoutRoutes() chi.Router {
	r := chi.NewRouter()
	// PUT, not POST: favoriting twice must not make two rows, and re-confirming a stale
	// endorsement is the same gesture as making it.
	r.Put("/", h.Favorite)
	r.Delete("/", h.Unfavorite)
	r.Get("/", h.ListForLoadout)
	return r
}

// ScopeRoutes mounts at /favorites and serves a scope's shelf.
func (h *FavoriteHandler) ScopeRoutes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.ListForScope)
	return r
}

func (h *FavoriteHandler) Favorite(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ScopeType core.FavoriteScope `json:"scope_type"`
		ScopeID   string             `json:"scope_id"`
		Note      string             `json:"note"`
	}
	if err := decode(r, &req); err != nil {
		writeError(w, err)
		return
	}
	// The overwhelmingly common case is "bookmark this for me", so an omitted scope means
	// the acting profile rather than an error.
	profileID := auth.ProfileID(r.Context())
	if req.ScopeType == "" {
		req.ScopeType = core.FavoriteProfile
	}
	if req.ScopeID == "" && req.ScopeType == core.FavoriteProfile {
		req.ScopeID = profileID
	}

	view, err := h.favorites.Favorite(r.Context(), profileID, req.ScopeType, req.ScopeID,
		chi.URLParam(r, "loadoutID"), req.Note)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (h *FavoriteHandler) Unfavorite(w http.ResponseWriter, r *http.Request) {
	profileID := auth.ProfileID(r.Context())
	scopeType, scopeID := favoriteScopeFrom(r, profileID)

	if err := h.favorites.Unfavorite(r.Context(), profileID, scopeType, scopeID, chi.URLParam(r, "loadoutID")); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}

// ListForLoadout answers "who vouches for this?" — community endorsements only.
func (h *FavoriteHandler) ListForLoadout(w http.ResponseWriter, r *http.Request) {
	views, err := h.favorites.ListForLoadout(r.Context(), chi.URLParam(r, "loadoutID"), auth.ProfileID(r.Context()))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, views)
}

// ListForScope is a shelf: this community's endorsements, or your own bookmarks.
func (h *FavoriteHandler) ListForScope(w http.ResponseWriter, r *http.Request) {
	profileID := auth.ProfileID(r.Context())
	scopeType, scopeID := favoriteScopeFrom(r, profileID)

	views, err := h.favorites.ListForScope(r.Context(), profileID, scopeType, scopeID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, views)
}

// favoriteScopeFrom reads the scope from query params, defaulting to the caller's own
// bookmarks. The service still authorizes it; this only decides what was asked for.
func favoriteScopeFrom(r *http.Request, profileID string) (core.FavoriteScope, string) {
	scopeType := core.FavoriteScope(r.URL.Query().Get("scope_type"))
	scopeID := r.URL.Query().Get("scope_id")
	if scopeType == "" {
		scopeType = core.FavoriteProfile
	}
	if scopeID == "" && scopeType == core.FavoriteProfile {
		scopeID = profileID
	}
	return scopeType, scopeID
}

package handlers

import (
	"fmt"
	"net/http"

	"github.com/gmccloskey/loadouts/backend/internal/auth"
	"github.com/gmccloskey/loadouts/backend/internal/core"
	"github.com/gmccloskey/loadouts/backend/internal/service"
	"github.com/go-chi/chi/v5"
)

// IdentityHandler exposes users, profiles, and "who am I".
type IdentityHandler struct {
	identity  *service.IdentityService
	loadouts  *service.LoadoutService
	community *service.CommunityService
}

func NewIdentityHandler(identity *service.IdentityService, loadouts *service.LoadoutService, community *service.CommunityService) *IdentityHandler {
	return &IdentityHandler{identity: identity, loadouts: loadouts, community: community}
}

// UserRoutes mounts /users.
func (h *IdentityHandler) UserRoutes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.ListUsers)
	r.Post("/", h.CreateUser)
	r.Get("/{userID}", h.GetUser)
	r.Get("/{userID}/profiles", h.ListUserProfiles)
	return r
}

// ProfileRoutes mounts /profiles.
func (h *IdentityHandler) ProfileRoutes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.ListProfiles)
	r.Post("/", h.CreateProfile)
	r.Get("/me", h.Me)
	r.Get("/{handle}", h.GetProfile)
	r.Get("/{handle}/loadouts", h.ProfileLoadouts)
	r.Get("/{handle}/communities", h.ProfileCommunities)
	return r
}

func (h *IdentityHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.identity.ListUsers(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, users)
}

func (h *IdentityHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email       string `json:"email"`
		DisplayName string `json:"display_name"`
		// Handle optionally bootstraps a first profile in the same call, which is what
		// every real signup flow wants anyway.
		Handle string `json:"handle"`
	}
	if err := decode(r, &req); err != nil {
		writeError(w, err)
		return
	}

	user, err := h.identity.CreateUser(r.Context(), req.Email, req.DisplayName)
	if err != nil {
		writeError(w, err)
		return
	}

	response := map[string]interface{}{"user": user}
	if req.Handle != "" {
		profile, err := h.identity.CreateProfile(r.Context(), core.Profile{
			UserID:      user.ID,
			Handle:      req.Handle,
			DisplayName: req.DisplayName,
		})
		if err != nil {
			writeError(w, err)
			return
		}
		response["profile"] = profile
	}
	writeJSON(w, http.StatusCreated, response)
}

func (h *IdentityHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	user, err := h.identity.GetUser(r.Context(), chi.URLParam(r, "userID"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, user)
}

func (h *IdentityHandler) ListUserProfiles(w http.ResponseWriter, r *http.Request) {
	profiles, err := h.identity.ListProfiles(r.Context(), chi.URLParam(r, "userID"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, profiles)
}

func (h *IdentityHandler) ListProfiles(w http.ResponseWriter, r *http.Request) {
	profiles, err := h.identity.ListProfiles(r.Context(), r.URL.Query().Get("user_id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, profiles)
}

func (h *IdentityHandler) CreateProfile(w http.ResponseWriter, r *http.Request) {
	var profile core.Profile
	if err := decode(r, &profile); err != nil {
		writeError(w, err)
		return
	}
	created, err := h.identity.CreateProfile(r.Context(), profile)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

// Me echoes the profile resolved from the auth header, which is how the client bootstraps.
func (h *IdentityHandler) Me(w http.ResponseWriter, r *http.Request) {
	profile, ok := auth.CurrentProfile(r.Context())
	if !ok {
		writeError(w, fmt.Errorf("%w: no acting profile; send the X-Profile-ID header", core.ErrForbidden))
		return
	}
	writeJSON(w, http.StatusOK, profile)
}

func (h *IdentityHandler) GetProfile(w http.ResponseWriter, r *http.Request) {
	profile, err := h.identity.Resolve(r.Context(), chi.URLParam(r, "handle"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, profile)
}

func (h *IdentityHandler) ProfileLoadouts(w http.ResponseWriter, r *http.Request) {
	profile, err := h.identity.Resolve(r.Context(), chi.URLParam(r, "handle"))
	if err != nil {
		writeError(w, err)
		return
	}
	// Discover filters by visibility, so a visitor only sees the shareable subset.
	summaries, err := h.loadouts.Discover(r.Context(), core.DiscoverQuery{
		ProfileID: profile.ID,
		Limit:     queryInt(r, "limit", 50),
	}, auth.ProfileID(r.Context()))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, summaries)
}

func (h *IdentityHandler) ProfileCommunities(w http.ResponseWriter, r *http.Request) {
	profile, err := h.identity.Resolve(r.Context(), chi.URLParam(r, "handle"))
	if err != nil {
		writeError(w, err)
		return
	}
	communities, err := h.community.ListForProfile(r.Context(), profile.ID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, communities)
}

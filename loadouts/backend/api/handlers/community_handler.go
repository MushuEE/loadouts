package handlers

import (
	"fmt"
	"net/http"

	"github.com/gmccloskey/loadouts/backend/internal/auth"
	"github.com/gmccloskey/loadouts/backend/internal/core"
	"github.com/gmccloskey/loadouts/backend/internal/service"
	"github.com/go-chi/chi/v5"
)

// CommunityHandler exposes communities, membership, community templates/loadouts, and the
// community metadata layer on items.
type CommunityHandler struct {
	community *service.CommunityService
	templates *service.TemplateService
	loadouts  *service.LoadoutService
}

func NewCommunityHandler(community *service.CommunityService, templates *service.TemplateService, loadouts *service.LoadoutService) *CommunityHandler {
	return &CommunityHandler{community: community, templates: templates, loadouts: loadouts}
}

func (h *CommunityHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.List)
	r.Post("/", h.Create)
	r.Route("/{slug}", func(r chi.Router) {
		r.Get("/", h.Get)
		r.Post("/join", h.Join)
		r.Post("/leave", h.Leave)
		r.Get("/members", h.Members)
		r.Get("/templates", h.Templates)
		r.Get("/loadouts", h.Loadouts)
		r.Put("/items/{itemID}/layer", h.SetItemLayer)
	})
	return r
}

func (h *CommunityHandler) List(w http.ResponseWriter, r *http.Request) {
	communities, err := h.community.List(r.Context(), r.URL.Query().Get("q"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, communities)
}

func (h *CommunityHandler) Create(w http.ResponseWriter, r *http.Request) {
	var community core.Community
	if err := decode(r, &community); err != nil {
		writeError(w, err)
		return
	}
	created, err := h.community.Create(r.Context(), auth.ProfileID(r.Context()), community)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

// communityView adds the caller's own role, which the UI needs to decide whether to show
// "Join" or the admin affordances.
type communityView struct {
	Community   core.Community  `json:"community"`
	ViewerRole  core.MemberRole `json:"viewer_role"`
	MemberCount int             `json:"member_count"`
}

func (h *CommunityHandler) Get(w http.ResponseWriter, r *http.Request) {
	community, err := h.community.Get(r.Context(), chi.URLParam(r, "slug"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, communityView{
		Community:   community,
		ViewerRole:  h.community.RoleOf(r.Context(), community.ID, auth.ProfileID(r.Context())),
		MemberCount: community.MemberCount,
	})
}

func (h *CommunityHandler) Join(w http.ResponseWriter, r *http.Request) {
	community, profileID, err := h.resolveActor(r)
	if err != nil {
		writeError(w, err)
		return
	}
	membership, err := h.community.Join(r.Context(), community.ID, profileID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, membership)
}

func (h *CommunityHandler) Leave(w http.ResponseWriter, r *http.Request) {
	community, profileID, err := h.resolveActor(r)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := h.community.Leave(r.Context(), community.ID, profileID); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}

func (h *CommunityHandler) Members(w http.ResponseWriter, r *http.Request) {
	community, err := h.community.Get(r.Context(), chi.URLParam(r, "slug"))
	if err != nil {
		writeError(w, err)
		return
	}
	members, err := h.community.Members(r.Context(), community.ID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, members)
}

func (h *CommunityHandler) Templates(w http.ResponseWriter, r *http.Request) {
	community, err := h.community.Get(r.Context(), chi.URLParam(r, "slug"))
	if err != nil {
		writeError(w, err)
		return
	}
	templates, err := h.templates.List(r.Context(), core.TemplateQuery{CommunityID: community.ID})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, templates)
}

func (h *CommunityHandler) Loadouts(w http.ResponseWriter, r *http.Request) {
	community, err := h.community.Get(r.Context(), chi.URLParam(r, "slug"))
	if err != nil {
		writeError(w, err)
		return
	}
	summaries, err := h.loadouts.Discover(r.Context(), core.DiscoverQuery{
		CommunityID: community.ID,
		Text:        r.URL.Query().Get("q"),
		Limit:       queryInt(r, "limit", 50),
	}, auth.ProfileID(r.Context()))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, summaries)
}

// SetItemLayer attaches community-scoped metadata to a global item (admin only).
func (h *CommunityHandler) SetItemLayer(w http.ResponseWriter, r *http.Request) {
	community, profileID, err := h.resolveActor(r)
	if err != nil {
		writeError(w, err)
		return
	}

	var req struct {
		Metadata core.Metadata `json:"metadata"`
	}
	if err := decode(r, &req); err != nil {
		writeError(w, err)
		return
	}

	layer, err := h.community.SetItemLayer(r.Context(), profileID, community.ID, chi.URLParam(r, "itemID"), req.Metadata)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, layer)
}

// resolveActor loads the community from the URL and requires an authenticated profile.
func (h *CommunityHandler) resolveActor(r *http.Request) (core.Community, string, error) {
	community, err := h.community.Get(r.Context(), chi.URLParam(r, "slug"))
	if err != nil {
		return core.Community{}, "", err
	}
	profileID := auth.ProfileID(r.Context())
	if profileID == "" {
		return core.Community{}, "", fmt.Errorf("%w: send the X-Profile-ID header", core.ErrForbidden)
	}
	return community, profileID, nil
}

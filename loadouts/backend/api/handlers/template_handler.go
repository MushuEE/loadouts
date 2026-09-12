package handlers

import (
	"net/http"

	"github.com/gmccloskey/loadouts/backend/internal/auth"
	"github.com/gmccloskey/loadouts/backend/internal/core"
	"github.com/gmccloskey/loadouts/backend/internal/service"
	"github.com/go-chi/chi/v5"
)

// TemplateHandler exposes templates and their immutable versions.
type TemplateHandler struct {
	templates *service.TemplateService
}

func NewTemplateHandler(templates *service.TemplateService) *TemplateHandler {
	return &TemplateHandler{templates: templates}
}

func (h *TemplateHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.List)
	r.Post("/", h.Create)
	r.Route("/{templateID}", func(r chi.Router) {
		r.Get("/", h.Get)
		r.Get("/versions", h.ListVersions)
		r.Post("/versions", h.PublishVersion)
	})
	return r
}

func (h *TemplateHandler) List(w http.ResponseWriter, r *http.Request) {
	q := core.TemplateQuery{
		Text:        r.URL.Query().Get("q"),
		OwnerType:   core.OwnerType(r.URL.Query().Get("owner_type")),
		OwnerID:     r.URL.Query().Get("owner_id"),
		CommunityID: r.URL.Query().Get("community_id"),
	}
	details, err := h.templates.List(r.Context(), q)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, details)
}

// createTemplateRequest keeps the slot payload separate from the template record, mirroring
// the storage split between the template and its versions.
type createTemplateRequest struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	OwnerType   core.OwnerType `json:"owner_type"`
	CommunityID string         `json:"community_id"`
	IsPublic    bool           `json:"is_public"`
	Slots       core.SlotList  `json:"slots"`
	Changelog   string         `json:"changelog"`
}

func (h *TemplateHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createTemplateRequest
	if err := decode(r, &req); err != nil {
		writeError(w, err)
		return
	}

	tmpl := core.Template{
		Name:        req.Name,
		Description: req.Description,
		OwnerType:   req.OwnerType,
		CommunityID: req.CommunityID,
		IsPublic:    req.IsPublic,
	}
	detail, err := h.templates.Create(r.Context(), auth.ProfileID(r.Context()), tmpl, req.Slots, req.Changelog)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, detail)
}

func (h *TemplateHandler) Get(w http.ResponseWriter, r *http.Request) {
	detail, err := h.templates.Detail(r.Context(), chi.URLParam(r, "templateID"), queryInt(r, "version", 0))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (h *TemplateHandler) ListVersions(w http.ResponseWriter, r *http.Request) {
	detail, err := h.templates.Detail(r.Context(), chi.URLParam(r, "templateID"), 0)
	if err != nil {
		writeError(w, err)
		return
	}

	versions := make([]core.TemplateVersion, 0, len(detail.Versions))
	for _, number := range detail.Versions {
		v, err := h.templates.Detail(r.Context(), detail.Template.ID, number)
		if err != nil {
			continue
		}
		versions = append(versions, v.Version)
	}
	writeJSON(w, http.StatusOK, versions)
}

// PublishVersion appends a new immutable version; existing loadouts stay pinned to theirs.
func (h *TemplateHandler) PublishVersion(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Slots     core.SlotList `json:"slots"`
		Changelog string        `json:"changelog"`
	}
	if err := decode(r, &req); err != nil {
		writeError(w, err)
		return
	}

	detail, err := h.templates.PublishVersion(r.Context(), auth.ProfileID(r.Context()), chi.URLParam(r, "templateID"), req.Slots, req.Changelog)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, detail)
}

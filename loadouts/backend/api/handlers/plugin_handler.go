package handlers

import (
	"net/http"

	"github.com/gmccloskey/loadouts/backend/internal/auth"
	"github.com/gmccloskey/loadouts/backend/internal/core"
	"github.com/gmccloskey/loadouts/backend/internal/service"
	"github.com/go-chi/chi/v5"
)

// PluginHandler exposes the plugin directory, installs, rendering, and plugin storage.
//
// Every authorization decision lives in the service, so these handlers stay thin: they
// decode, call, and map errors. That is deliberate - the rules that keep an untrusted
// plugin from reading a private loadout should not be something a new endpoint can forget
// to repeat.
type PluginHandler struct {
	svc *service.PluginService
}

func NewPluginHandler(svc *service.PluginService) *PluginHandler {
	return &PluginHandler{svc: svc}
}

func (h *PluginHandler) Routes() chi.Router {
	r := chi.NewRouter()

	// Directory
	r.Get("/", h.List)
	r.Post("/", h.Create)
	r.Get("/{pluginID}", h.Get)
	r.Get("/{pluginID}/versions", h.Versions)
	r.Post("/{pluginID}/versions", h.PublishVersion)
	r.Patch("/{pluginID}/visibility", h.SetVisibility)

	// Installs
	r.Post("/installs", h.Install)
	r.Get("/installs", h.ListInstalls)
	r.Patch("/installs/{installID}", h.UpdateInstall)
	r.Delete("/installs/{installID}", h.Uninstall)

	// Rendering
	r.Get("/render", h.Render)

	// Plugin-namespaced storage
	r.Get("/{pluginID}/data", h.ListData)
	r.Put("/{pluginID}/data/{key}", h.PutDatum)
	r.Delete("/{pluginID}/data/{key}", h.DeleteDatum)

	return r
}

// List returns the plugin directory. Unlisted plugins are hidden from everyone but their
// author.
func (h *PluginHandler) List(w http.ResponseWriter, r *http.Request) {
	viewer := auth.ProfileID(r.Context())
	q := core.PluginQuery{
		Search:      r.URL.Query().Get("q"),
		OwnerType:   core.OwnerType(r.URL.Query().Get("owner_type")),
		OwnerID:     r.URL.Query().Get("owner_id"),
		Surface:     core.Surface(r.URL.Query().Get("surface")),
		PublicOnly:  true,
		IncludeMine: viewer,
	}
	plugins, err := h.svc.List(r.Context(), q)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"plugins":  plugins,
		"surfaces": core.Surfaces(),
	})
}

// Create publishes a new plugin at version 1.
func (h *PluginHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req service.PublishRequest
	if err := decode(r, &req); err != nil {
		writeError(w, err)
		return
	}
	detail, err := h.svc.Create(r.Context(), auth.ProfileID(r.Context()), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, detail)
}

// Get returns one plugin. `?version=` pins a specific published version.
func (h *PluginHandler) Get(w http.ResponseWriter, r *http.Request) {
	detail, err := h.svc.Detail(r.Context(), chi.URLParam(r, "pluginID"), queryInt(r, "version", 0))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

// Versions lists a plugin's published history.
func (h *PluginHandler) Versions(w http.ResponseWriter, r *http.Request) {
	versions, err := h.svc.Versions(r.Context(), chi.URLParam(r, "pluginID"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"versions": versions})
}

// PublishVersion appends an immutable version.
func (h *PluginHandler) PublishVersion(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Manifest  core.PluginManifest `json:"manifest"`
		Changelog string              `json:"changelog"`
	}
	if err := decode(r, &req); err != nil {
		writeError(w, err)
		return
	}
	detail, err := h.svc.PublishVersion(r.Context(), auth.ProfileID(r.Context()),
		chi.URLParam(r, "pluginID"), req.Manifest, req.Changelog)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, detail)
}

// SetVisibility lists or unlists a plugin.
func (h *PluginHandler) SetVisibility(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IsPublic bool `json:"is_public"`
	}
	if err := decode(r, &req); err != nil {
		writeError(w, err)
		return
	}
	plugin, err := h.svc.SetVisibility(r.Context(), auth.ProfileID(r.Context()),
		chi.URLParam(r, "pluginID"), req.IsPublic)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, plugin)
}

// Install enables a plugin in a scope, recording the capabilities the user approved.
func (h *PluginHandler) Install(w http.ResponseWriter, r *http.Request) {
	var req service.InstallRequest
	if err := decode(r, &req); err != nil {
		writeError(w, err)
		return
	}
	view, err := h.svc.Install(r.Context(), auth.ProfileID(r.Context()), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, view)
}

// ListInstalls returns what is installed in a scope.
func (h *PluginHandler) ListInstalls(w http.ResponseWriter, r *http.Request) {
	scopeType := r.URL.Query().Get("scope_type")
	scopeID := r.URL.Query().Get("scope_id")
	if scopeType == core.ScopeProfile && scopeID == "" {
		// The common case: "what have I installed?"
		scopeID = auth.ProfileID(r.Context())
	}
	views, err := h.svc.ListInstalls(r.Context(), scopeType, scopeID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"installs": views})
}

// UpdateInstall toggles an install or replaces its settings.
func (h *PluginHandler) UpdateInstall(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Enabled  *bool         `json:"enabled"`
		Settings core.Metadata `json:"settings"`
	}
	if err := decode(r, &req); err != nil {
		writeError(w, err)
		return
	}

	actor := auth.ProfileID(r.Context())
	installID := chi.URLParam(r, "installID")

	var view service.InstallView
	var err error
	if req.Settings != nil {
		view, err = h.svc.UpdateSettings(r.Context(), actor, installID, req.Settings)
		if err != nil {
			writeError(w, err)
			return
		}
	}
	if req.Enabled != nil {
		view, err = h.svc.SetEnabled(r.Context(), actor, installID, *req.Enabled)
		if err != nil {
			writeError(w, err)
			return
		}
	}
	if req.Settings == nil && req.Enabled == nil {
		writeError(w, core.ErrInvalid)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

// Uninstall removes an install. Stored plugin data survives, so reinstalling restores it.
func (h *PluginHandler) Uninstall(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Uninstall(r.Context(), auth.ProfileID(r.Context()), chi.URLParam(r, "installID")); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}

// Render evaluates every view installed on one surface of one thing.
func (h *PluginHandler) Render(w http.ResponseWriter, r *http.Request) {
	req := service.RenderRequest{
		Surface:     core.Surface(r.URL.Query().Get("surface")),
		LoadoutID:   r.URL.Query().Get("loadout_id"),
		ItemID:      r.URL.Query().Get("item_id"),
		CommunityID: r.URL.Query().Get("community_id"),
	}
	views, err := h.svc.RenderSurface(r.Context(), auth.ProfileID(r.Context()), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"views": views})
}

// ListData returns what a plugin has stored against a scope.
func (h *PluginHandler) ListData(w http.ResponseWriter, r *http.Request) {
	data, err := h.svc.ListData(r.Context(), auth.ProfileID(r.Context()),
		chi.URLParam(r, "pluginID"),
		r.URL.Query().Get("scope_type"),
		r.URL.Query().Get("scope_id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": data})
}

// PutDatum writes one plugin-namespaced value.
func (h *PluginHandler) PutDatum(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ScopeType string        `json:"scope_type"`
		ScopeID   string        `json:"scope_id"`
		Value     core.Metadata `json:"value"`
	}
	if err := decode(r, &req); err != nil {
		writeError(w, err)
		return
	}
	datum, err := h.svc.PutDatum(r.Context(), auth.ProfileID(r.Context()),
		chi.URLParam(r, "pluginID"), req.ScopeType, req.ScopeID, chi.URLParam(r, "key"), req.Value)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, datum)
}

// DeleteDatum removes one stored value.
func (h *PluginHandler) DeleteDatum(w http.ResponseWriter, r *http.Request) {
	err := h.svc.DeleteDatum(r.Context(), auth.ProfileID(r.Context()),
		chi.URLParam(r, "pluginID"),
		r.URL.Query().Get("scope_type"),
		r.URL.Query().Get("scope_id"),
		chi.URLParam(r, "key"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}

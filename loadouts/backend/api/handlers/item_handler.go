package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gmccloskey/loadouts/backend/internal/auth"
	"github.com/gmccloskey/loadouts/backend/internal/core"
	"github.com/gmccloskey/loadouts/backend/internal/service"
	"github.com/go-chi/chi/v5"
)

type ItemHandler struct {
	svc *service.InventoryService
	// community lets ?community= accept a slug as well as an ID, matching the rest of the API.
	community *service.CommunityService
}

func NewItemHandler(svc *service.InventoryService, community *service.CommunityService) *ItemHandler {
	return &ItemHandler{svc: svc, community: community}
}

// resolveCommunityID turns a slug or ID from the query string into a community ID.
func (h *ItemHandler) resolveCommunityID(r *http.Request) string {
	raw := r.URL.Query().Get("community")
	if raw == "" || h.community == nil {
		return raw
	}
	community, err := h.community.Get(r.Context(), raw)
	if err != nil {
		return raw
	}
	return community.ID
}

func (h *ItemHandler) Routes() chi.Router {
	r := chi.NewRouter()

	r.Get("/", h.List)
	r.Post("/", h.Create)
	// Static, so chi matches it ahead of /{itemID}.
	r.Get("/tags", h.PopularTags)
	r.Get("/search", h.Search)
	r.Route("/{itemID}", func(r chi.Router) {
		r.Get("/", h.Get)
		r.Post("/metadata", h.UpdateMetadata) // Legacy compat shim
		r.Get("/layers/profile", h.GetProfileLayer)
		r.Put("/layers/profile", h.SetProfileLayer)
		r.Get("/tags", h.GetTags)
		r.Put("/tags", h.SetTags)
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

// Get resolves an item through every applicable metadata layer.
//
//	?community=<id|slug>  applies that community's layer
//	?owner=<profileID>    views another profile's public layer (private stays hidden)
func (h *ItemHandler) Get(w http.ResponseWriter, r *http.Request) {
	itemID := chi.URLParam(r, "itemID")
	viewerID := auth.ProfileID(r.Context())

	lctx := core.LayerContext{
		ViewerProfileID: viewerID,
		OwnerProfileID:  defaultQuery(r.URL.Query().Get("owner"), viewerID),
		CommunityID:     h.resolveCommunityID(r),
	}

	item, err := h.svc.ResolveItem(r.Context(), itemID, lctx)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

// GetProfileLayer returns the raw (unmerged) profile layer, useful for edit forms.
func (h *ItemHandler) GetProfileLayer(w http.ResponseWriter, r *http.Request) {
	viewerID := auth.ProfileID(r.Context())
	ownerID := defaultQuery(r.URL.Query().Get("owner"), viewerID)

	layer, err := h.svc.GetProfileLayer(r.Context(), ownerID, chi.URLParam(r, "itemID"), viewerID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, layer)
}

// SetProfileLayer writes the acting profile's public overrides and private notes.
func (h *ItemHandler) SetProfileLayer(w http.ResponseWriter, r *http.Request) {
	profileID := auth.ProfileID(r.Context())
	if profileID == "" {
		writeError(w, fmt.Errorf("%w: send the X-Profile-ID header", core.ErrForbidden))
		return
	}

	var req struct {
		CustomImageURL  string        `json:"custom_image_url"`
		PublicMetadata  core.Metadata `json:"public_metadata"`
		PrivateMetadata core.Metadata `json:"private_metadata"`
	}
	if err := decode(r, &req); err != nil {
		writeError(w, err)
		return
	}

	layer, err := h.svc.SetProfileLayer(r.Context(), core.ProfileItemLayer{
		ProfileID:       profileID,
		ItemID:          chi.URLParam(r, "itemID"),
		CustomImageURL:  req.CustomImageURL,
		PublicMetadata:  req.PublicMetadata,
		PrivateMetadata: req.PrivateMetadata,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, layer)
}

// GetTags returns the acting profile's tags for an item and everyone's, aggregated.
func (h *ItemHandler) GetTags(w http.ResponseWriter, r *http.Request) {
	tags, err := h.svc.ItemTags(r.Context(), auth.ProfileID(r.Context()), chi.URLParam(r, "itemID"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, tags)
}

// PopularTags lists tags by how many profiles use them, for suggestions.
//
//	?prefix=war   only tags starting with "war"
//	?limit=20     at most 20 (default 20)
func (h *ItemHandler) PopularTags(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	tags, err := h.svc.PopularTags(r.Context(), r.URL.Query().Get("prefix"), limit)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"tags": tags})
}

// Search finds gear. ?scope=mine|everyone picks whose gear (and whose tags) to search;
// ?tags=a,b must all match; ?category narrows; ?q is free text, where #words count as tags.
func (h *ItemHandler) Search(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	query := core.GearQuery{
		Text:      q.Get("q"),
		Category:  q.Get("category"),
		Tags:      splitTags(q.Get("tags")),
		Scope:     core.ParseGearScope(q.Get("scope")),
		ProfileID: auth.ProfileID(r.Context()),
		Limit:     limit,
	}
	if query.Scope == core.ScopeMine && query.ProfileID == "" {
		writeError(w, fmt.Errorf("%w: searching your gear needs a profile", core.ErrInvalid))
		return
	}
	res, err := h.svc.SearchGear(r.Context(), query)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// SetTags replaces the acting profile's tags for an item.
func (h *ItemHandler) SetTags(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Tags []string `json:"tags"`
	}
	if err := decode(r, &req); err != nil {
		writeError(w, err)
		return
	}
	tags, err := h.svc.SetItemTags(r.Context(), auth.ProfileID(r.Context()), chi.URLParam(r, "itemID"), req.Tags)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"tags": tags})
}

func (h *ItemHandler) UpdateMetadata(w http.ResponseWriter, r *http.Request) {
	itemID := chi.URLParam(r, "itemID")
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		http.Error(w, "X-User-ID header required", http.StatusUnauthorized)
		return
	}

	var req struct {
		CustomImageURL string        `json:"custom_image_url"`
		Overrides      core.Metadata `json:"overrides"`
		OpenData       core.Metadata `json:"open_data"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.svc.UpdateMetadata(r.Context(), userID, itemID, req.CustomImageURL, req.Overrides, req.OpenData); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

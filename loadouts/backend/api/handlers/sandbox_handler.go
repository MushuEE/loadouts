package handlers

import (
	"net/http"
	"strconv"

	"github.com/gmccloskey/loadouts/backend/internal/service"
	"github.com/go-chi/chi/v5"
)

// SandboxHandler serves plugin embed views as isolated documents.
//
// This route is deliberately outside /api/v1: it returns HTML, not JSON, and it is the
// one place where code somebody else wrote gets executed in a user's browser. The
// isolation comes from three things working together - the parent's sandbox attribute
// (allow-scripts without allow-same-origin, giving the frame an opaque origin), the CSP
// built from the install's granted capabilities, and the fact that the frame has no
// credentials and must ask the parent for anything privileged.
type SandboxHandler struct {
	svc *service.PluginService
}

func NewSandboxHandler(svc *service.PluginService) *SandboxHandler {
	return &SandboxHandler{svc: svc}
}

func (h *SandboxHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/plugins/{pluginID}/versions/{version}/views/{viewID}/frame", h.Frame)
	return r
}

// Frame serves one embed view.
func (h *SandboxHandler) Frame(w http.ResponseWriter, r *http.Request) {
	version, err := strconv.Atoi(chi.URLParam(r, "version"))
	if err != nil {
		http.Error(w, "bad version", http.StatusBadRequest)
		return
	}

	doc, err := h.svc.EmbedDocumentFor(r.Context(),
		chi.URLParam(r, "pluginID"),
		version,
		chi.URLParam(r, "viewID"),
		r.URL.Query().Get("install"),
	)
	if err != nil {
		// Plain text, because an error page here is rendered inside someone's frame.
		http.Error(w, err.Error(), statusFor(err))
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Security-Policy", doc.CSP)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	// The document is per-install and may embed install settings, so it must not be
	// cached by anything shared.
	w.Header().Set("Cache-Control", "private, no-store")
	// The CORS wildcard the API sets would be actively harmful on a document response.
	w.Header().Del("Access-Control-Allow-Origin")

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(doc.HTML))
}

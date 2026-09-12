package service

import (
	"context"
	"fmt"
	"html"
	"strings"

	"github.com/gmccloskey/loadouts/backend/internal/core"
)

// The sandbox: how author-written HTML and JavaScript get to run without being trusted.
//
// An embed view is served as its own document into an iframe the host marks
// sandbox="allow-scripts" and deliberately NOT allow-same-origin. Those two together
// would void the sandbox entirely; omitting allow-same-origin gives the frame an opaque
// origin, which means no access to the parent, no cookies, and no localStorage.
//
// The frame therefore has no credentials and cannot call our API at all. Everything it
// needs arrives by postMessage from the parent, which is the authenticated party. That is
// the point: a plugin cannot make a request as the user, it can only ask the page to.
//
// On top of that, the capabilities the install granted become a Content-Security-Policy,
// so the network allowlist is enforced by the browser rather than by our good intentions.

// EmbedDocument is a rendered sandbox page plus the policy it must be served under.
type EmbedDocument struct {
	HTML   string
	CSP    string
	Title  string
	Height int
}

// EmbedDocumentFor assembles the sandbox page for one installed embed view.
func (s *PluginService) EmbedDocumentFor(ctx context.Context, pluginID string, version int, viewID, installID string) (EmbedDocument, error) {
	install, err := s.store.GetPluginInstall(ctx, installID)
	if err != nil {
		return EmbedDocument{}, fmt.Errorf("%w: install %s", core.ErrNotFound, installID)
	}
	// The frame is addressed by (plugin, version, view) but authorised by the install,
	// so a URL cannot be edited into serving a version nobody installed.
	if install.PluginID != pluginID || install.Version != version {
		return EmbedDocument{}, fmt.Errorf("%w: this install does not serve %s v%d", core.ErrForbidden, pluginID, version)
	}
	if !install.Enabled {
		return EmbedDocument{}, fmt.Errorf("%w: this plugin is disabled", core.ErrForbidden)
	}

	pv, err := s.store.GetPluginVersion(ctx, pluginID, version)
	if err != nil {
		return EmbedDocument{}, fmt.Errorf("%w: plugin %s v%d", core.ErrNotFound, pluginID, version)
	}
	view, ok := pv.Manifest.ViewByID(viewID)
	if !ok {
		return EmbedDocument{}, fmt.Errorf("%w: view %q", core.ErrNotFound, viewID)
	}
	if view.Kind != core.ViewEmbed {
		return EmbedDocument{}, fmt.Errorf("%w: view %q is a widget, which the host renders itself", core.ErrInvalid, viewID)
	}

	caps := effectiveCaps(pv.Manifest.Capabilities, install)
	return EmbedDocument{
		HTML:   embedPage(view, caps),
		CSP:    contentSecurityPolicy(caps, s.appOrigin),
		Title:  view.Title,
		Height: view.Height,
	}, nil
}

// contentSecurityPolicy turns granted capabilities into a browser-enforced policy.
//
// The baseline is default-src 'none': a plugin that was granted no network access can
// reach nothing at all. Each granted host is then added to the directives an embed
// plausibly needs it for.
func contentSecurityPolicy(caps core.Capabilities, appOrigin string) string {
	hosts := make([]string, 0, len(caps.Network))
	for _, h := range caps.Network {
		hosts = append(hosts, normalizeCSPHost(h))
	}
	joined := strings.Join(hosts, " ")

	directives := []string{
		"default-src 'none'",
		"base-uri 'none'",
		"form-action 'none'",
		// Author scripts and styles are inline in the manifest, so they need
		// 'unsafe-inline'. This is safe here only because the frame's origin is opaque:
		// there is no same-origin data for injected script to reach.
		strings.TrimSpace("script-src 'unsafe-inline' " + joined),
		strings.TrimSpace("style-src 'unsafe-inline' " + joined),
		strings.TrimSpace("img-src data: blob: " + joined),
		strings.TrimSpace("font-src data: " + joined),
	}
	if joined != "" {
		directives = append(directives,
			"connect-src "+joined,
			"frame-src "+joined,
		)
	}
	// Only our own app may frame this page, so a sandbox URL cannot be embedded
	// elsewhere and pointed at somebody's data.
	directives = append(directives, "frame-ancestors "+defaultString(appOrigin, "'self'"))
	return strings.Join(directives, "; ")
}

// normalizeCSPHost accepts "maps.googleapis.com" or "https://maps.googleapis.com" and
// always emits an https origin. Manifest validation already rejected hosts containing
// whitespace or quotes, so this cannot inject a directive.
func normalizeCSPHost(host string) string {
	host = strings.TrimSpace(host)
	if strings.HasPrefix(host, "https://") || strings.HasPrefix(host, "http://") {
		return host
	}
	return "https://" + host
}

// embedPage wraps author HTML in the bridge that lets it talk to the host.
func embedPage(view core.PluginView, caps core.Capabilities) string {
	var sb strings.Builder
	sb.WriteString("<!doctype html>\n<html lang=\"en\">\n<head>\n")
	sb.WriteString("<meta charset=\"utf-8\">\n")
	sb.WriteString("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">\n")
	sb.WriteString("<title>" + html.EscapeString(view.Title) + "</title>\n")
	sb.WriteString("<style>" + baseFrameStyles + "</style>\n")
	sb.WriteString("<script>" + bridgeScript + "</script>\n")
	sb.WriteString("</head>\n<body>\n")
	// The author's document body, verbatim. It is not escaped, because running it is
	// the entire point; it is contained by the sandbox attribute and the CSP, not by
	// sanitisation.
	sb.WriteString(view.HTML)
	sb.WriteString("\n</body>\n</html>\n")
	_ = caps
	return sb.String()
}

const baseFrameStyles = `
:root { color-scheme: dark; }
html, body {
  margin: 0; padding: 0;
  background: transparent;
  color: #e7e5e4;
  font: 14px/1.5 ui-sans-serif, system-ui, -apple-system, "Segoe UI", sans-serif;
}
* { box-sizing: border-box; }
a { color: #fb923c; }
.loadouts-error {
  padding: 12px; border-radius: 8px;
  background: rgba(127,29,29,0.35); color: #fecaca;
}
`

// bridgeScript is the SDK a plugin author codes against.
//
// It exists because the frame has no credentials of its own. Every privileged action is
// a postMessage to the parent, which performs it as the signed-in user and posts the
// result back. An author never sees a token, and the host can refuse any request.
const bridgeScript = `
(function () {
  var pending = {};
  var seq = 0;
  var context = null;
  var readyCallbacks = [];

  function call(type, payload) {
    return new Promise(function (resolve, reject) {
      var id = 'req-' + (++seq);
      pending[id] = { resolve: resolve, reject: reject };
      parent.postMessage({ source: 'loadouts-plugin', id: id, type: type, payload: payload }, '*');
    });
  }

  window.addEventListener('message', function (event) {
    var msg = event.data;
    if (!msg || msg.source !== 'loadouts-host') return;

    if (msg.type === 'context') {
      context = msg.payload;
      var callbacks = readyCallbacks;
      readyCallbacks = [];
      callbacks.forEach(function (cb) {
        try { cb(context); } catch (e) { console.error(e); }
      });
      return;
    }

    var waiting = pending[msg.id];
    if (!waiting) return;
    delete pending[msg.id];
    if (msg.error) waiting.reject(new Error(msg.error));
    else waiting.resolve(msg.payload);
  });

  var Loadouts = {
    // ready(fn) runs fn with the render context, now or as soon as it arrives.
    ready: function (fn) {
      if (context) fn(context);
      else readyCallbacks.push(fn);
    },
    // context() is the host-provided data: loadout, settings, saved data, viewer.
    context: function () { return context; },
    // save(key, value) persists into this plugin's namespace for the current scope.
    // The host checks the storage capability and that the user may write here.
    save: function (key, value) { return call('data.put', { key: key, value: value }); },
    remove: function (key) { return call('data.delete', { key: key }); },
    // resize(px) asks the host to change the frame height.
    resize: function (px) { return call('resize', { height: px }); },
    // fail(message) renders an error the host can style consistently.
    fail: function (message) {
      document.body.innerHTML = '<div class="loadouts-error"></div>';
      document.body.firstChild.textContent = String(message);
    }
  };

  window.Loadouts = Loadouts;

  // Announce readiness once the document parses; the host replies with the context.
  document.addEventListener('DOMContentLoaded', function () {
    parent.postMessage({ source: 'loadouts-plugin', type: 'ready' }, '*');
  });
})();
`

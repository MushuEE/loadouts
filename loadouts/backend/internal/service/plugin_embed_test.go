package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/gmccloskey/loadouts/backend/internal/core"
)

// installedMapPlugin publishes the embed fixture and installs it for the user.
func installedMapPlugin(t *testing.T, h *harness, author, user core.Profile, caps []string) (core.PluginDetail, InstallView) {
	t.Helper()
	published := h.publish(t, author, "Trip Route", mapManifest())
	view, err := h.plugins.Install(context.Background(), user.ID, InstallRequest{
		PluginID: published.Plugin.ID, ScopeType: core.ScopeProfile, ScopeID: user.ID,
		GrantedCaps: caps, Settings: core.Metadata{"api_key": "sk"},
	})
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	return published, view
}

func TestEmbed_DocumentCarriesAuthorHTMLAndTheBridge(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	author := h.profile(t, "author")
	user := h.profile(t, "user")
	published, install := installedMapPlugin(t, h, author, user,
		[]string{"loadout:read", "storage:write", "network:maps.googleapis.com"})

	doc, err := h.plugins.EmbedDocumentFor(ctx, published.Plugin.ID, 1, "route", install.Install.ID)
	if err != nil {
		t.Fatalf("embed document: %v", err)
	}

	// The author's markup is served verbatim; containment is the sandbox and the CSP,
	// not sanitisation.
	if !strings.Contains(doc.HTML, `<div id=map>`) {
		t.Error("author html is missing from the document")
	}
	// The bridge must be present, because the frame has no other way to reach the host.
	if !strings.Contains(doc.HTML, "window.Loadouts") {
		t.Error("the plugin bridge is missing from the document")
	}
	if doc.Height != 420 {
		t.Errorf("height = %d, want 420", doc.Height)
	}
}

func TestEmbed_CSPReflectsGrantedNetworkAccess(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	author := h.profile(t, "author")
	user := h.profile(t, "user")
	published, install := installedMapPlugin(t, h, author, user,
		[]string{"loadout:read", "storage:write", "network:maps.googleapis.com"})

	doc, err := h.plugins.EmbedDocumentFor(ctx, published.Plugin.ID, 1, "route", install.Install.ID)
	if err != nil {
		t.Fatalf("embed document: %v", err)
	}

	for _, want := range []string{
		"default-src 'none'",
		"connect-src https://maps.googleapis.com",
		"script-src 'unsafe-inline' https://maps.googleapis.com",
		// Only our own app may frame the sandbox, so a frame URL cannot be embedded
		// elsewhere and pointed at somebody's data.
		"frame-ancestors https://app.test",
	} {
		if !strings.Contains(doc.CSP, want) {
			t.Errorf("csp = %q, want it to contain %q", doc.CSP, want)
		}
	}
}

func TestEmbed_UngrantedHostIsNotInThePolicy(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	author := h.profile(t, "author")
	user := h.profile(t, "user")

	// The manifest asks for the host but the install withholds it. The browser, not
	// our good intentions, is what stops the fetch.
	published, install := installedMapPlugin(t, h, author, user,
		[]string{"loadout:read", "storage:write", "network:maps.googleapis.com"})
	stored, _ := h.store.GetPluginInstall(ctx, install.Install.ID)
	stored.GrantedCaps = core.StringList{"loadout:read", "storage:write"}
	if err := h.store.UpsertPluginInstall(ctx, stored); err != nil {
		t.Fatalf("revoke: %v", err)
	}

	doc, err := h.plugins.EmbedDocumentFor(ctx, published.Plugin.ID, 1, "route", install.Install.ID)
	if err != nil {
		t.Fatalf("embed document: %v", err)
	}
	if strings.Contains(doc.CSP, "maps.googleapis.com") {
		t.Errorf("csp = %q, want the ungranted host absent", doc.CSP)
	}
	if !strings.Contains(doc.CSP, "default-src 'none'") {
		t.Errorf("csp = %q, want the deny-all baseline", doc.CSP)
	}
}

func TestEmbed_FrameIsAuthorizedByItsInstall(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	author := h.profile(t, "author")
	user := h.profile(t, "user")
	caps := []string{"loadout:read", "storage:write", "network:maps.googleapis.com"}
	published, install := installedMapPlugin(t, h, author, user, caps)

	// A URL cannot be edited into serving a version nobody installed.
	if _, err := h.plugins.PublishVersion(ctx, author.ID, published.Plugin.ID, mapManifest(), "v2"); err != nil {
		t.Fatalf("publish v2: %v", err)
	}
	if _, err := h.plugins.EmbedDocumentFor(ctx, published.Plugin.ID, 2, "route", install.Install.ID); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("serving an uninstalled version = %v, want ErrForbidden", err)
	}

	// A disabled install serves nothing.
	if _, err := h.plugins.SetEnabled(ctx, user.ID, install.Install.ID, false); err != nil {
		t.Fatalf("disable: %v", err)
	}
	if _, err := h.plugins.EmbedDocumentFor(ctx, published.Plugin.ID, 1, "route", install.Install.ID); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("serving a disabled install = %v, want ErrForbidden", err)
	}

	// An unknown install is a 404, not an accidental render.
	if _, err := h.plugins.EmbedDocumentFor(ctx, published.Plugin.ID, 1, "route", "pin_nope"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("unknown install = %v, want ErrNotFound", err)
	}
}

func TestEmbed_WidgetViewsAreNotServedAsFrames(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	author := h.profile(t, "author")
	user := h.profile(t, "user")

	published := h.publish(t, author, "Weight Breakdown", weightBreakdownManifest())
	install, err := h.plugins.Install(ctx, user.ID, InstallRequest{
		PluginID: published.Plugin.ID, ScopeType: core.ScopeProfile, ScopeID: user.ID,
		GrantedCaps: []string{"loadout:read", "items:read"},
	})
	if err != nil {
		t.Fatalf("install: %v", err)
	}

	// A widget is data the host draws itself; there is nothing to put in a frame.
	_, err = h.plugins.EmbedDocumentFor(ctx, published.Plugin.ID, 1, "breakdown", install.Install.ID)
	if !errors.Is(err, core.ErrInvalid) {
		t.Errorf("framing a widget = %v, want ErrInvalid", err)
	}
}

func TestEmbed_CSPHostNormalization(t *testing.T) {
	cases := []struct {
		name  string
		caps  core.Capabilities
		want  string
		avoid string
	}{
		{
			name: "bare hostname becomes an https origin",
			caps: core.Capabilities{Network: []string{"maps.googleapis.com"}},
			want: "https://maps.googleapis.com",
		},
		{
			name: "an explicit scheme is left alone",
			caps: core.Capabilities{Network: []string{"https://tiles.example.com"}},
			want: "https://tiles.example.com",
		},
		{
			name:  "no network access means no connect-src at all",
			caps:  core.Capabilities{},
			want:  "default-src 'none'",
			avoid: "connect-src",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			csp := contentSecurityPolicy(tc.caps, "https://app.test")
			if !strings.Contains(csp, tc.want) {
				t.Errorf("csp = %q, want %q", csp, tc.want)
			}
			if tc.avoid != "" && strings.Contains(csp, tc.avoid) {
				t.Errorf("csp = %q, want it to omit %q", csp, tc.avoid)
			}
		})
	}
}

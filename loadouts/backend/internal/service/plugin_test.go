package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/gmccloskey/loadouts/backend/internal/core"
)

// --- Fixtures ---

// weightBreakdownManifest is the canonical widget plugin: a pie chart of pack weight by
// category, reading the loadout and its items.
func weightBreakdownManifest() core.PluginManifest {
	return core.PluginManifest{
		APIVersion:   core.ManifestAPIVersion,
		Capabilities: core.Capabilities{ReadLoadout: true, ReadItems: true},
		Views: []core.PluginView{{
			ID:      "breakdown",
			Title:   "Weight breakdown",
			Surface: core.SurfaceLoadoutPanel,
			Kind:    core.ViewWidget,
			Widget: &core.WidgetSpec{
				Type:     core.WidgetPieChart,
				Source:   core.SourceLoadoutEntries,
				GroupBy:  "item.category",
				Label:    "item.category",
				Value:    "sum(item.weight_g * entry.quantity)",
				Format:   core.FormatGrams,
				SortBy:   "value",
				SortDesc: true,
			},
		}},
	}
}

// mapManifest is the canonical embed plugin: it needs a key and an outbound host, which
// is what forces per-install settings and the network allowlist to exist.
func mapManifest() core.PluginManifest {
	return core.PluginManifest{
		APIVersion: core.ManifestAPIVersion,
		Capabilities: core.Capabilities{
			ReadLoadout: true,
			Storage:     true,
			Network:     []string{"maps.googleapis.com"},
		},
		Settings: []core.SettingDefinition{
			{Key: "api_key", Label: "Maps API key", Type: core.SettingSecret, Required: true},
		},
		Views: []core.PluginView{{
			ID:      "route",
			Title:   "Trip route",
			Surface: core.SurfaceLoadoutSidebar,
			Kind:    core.ViewEmbed,
			HTML:    "<!doctype html><body><div id=map></div>",
			Height:  420,
		}},
	}
}

// pluginFixture publishes a plugin owned by author and returns it.
func (h *harness) publish(t *testing.T, author core.Profile, name string, manifest core.PluginManifest) core.PluginDetail {
	t.Helper()
	detail, err := h.plugins.Create(context.Background(), author.ID, PublishRequest{
		Name: name, Manifest: manifest, IsPublic: true,
	})
	if err != nil {
		t.Fatalf("publish %s: %v", name, err)
	}
	return detail
}

// loadoutFixture builds a two-category loadout owned by the given profile.
//
//	Tarp   240 g x1 = 240 g (shelter)
//	Stake   10 g x2 =  20 g (shelter)
//	Quilt  560 g x1 = 560 g (sleep)
func (h *harness) loadoutFixture(t *testing.T, owner core.Profile, communityID string) core.LoadoutDetail {
	t.Helper()
	ctx := context.Background()
	h.item(t, "tarp", "shelter", 240, 12000, false, nil)
	h.item(t, "stake", "shelter", 10, 500, false, nil)
	h.item(t, "quilt", "sleep", 560, 32000, false, nil)

	detail, err := h.loadouts.Create(ctx, owner.ID, CreateLoadoutRequest{
		Name: "UL Kit", CommunityID: communityID, Visibility: core.VisibilityPublic,
	})
	if err != nil {
		t.Fatalf("create loadout: %v", err)
	}
	detail, err = h.loadouts.ReplaceEntries(ctx, owner.ID, detail.Loadout.ID, []core.LoadoutEntry{
		{ItemID: "tarp", Quantity: 1},
		{ItemID: "stake", Quantity: 2},
		{ItemID: "quilt", Quantity: 1},
	})
	if err != nil {
		t.Fatalf("replace entries: %v", err)
	}
	return detail
}

// --- Publishing ---

func TestPlugin_PublishRejectsBrokenManifests(t *testing.T) {
	h := newHarness(t)
	author := h.profile(t, "author")
	ctx := context.Background()

	cases := []struct {
		name     string
		manifest core.PluginManifest
		want     string
	}{
		{
			"no views",
			core.PluginManifest{APIVersion: 1},
			"at least one view",
		},
		{
			"future api version",
			core.PluginManifest{APIVersion: 99, Views: []core.PluginView{{
				ID: "v", Surface: core.SurfaceLoadoutPanel, Kind: core.ViewEmbed, HTML: "<p>",
			}}},
			"newer than this host supports",
		},
		{
			"unknown surface",
			core.PluginManifest{APIVersion: 1, Views: []core.PluginView{{
				ID: "v", Surface: "everywhere", Kind: core.ViewEmbed, HTML: "<p>",
			}}},
			"unknown surface",
		},
		{
			"embed with no html",
			core.PluginManifest{APIVersion: 1, Views: []core.PluginView{{
				ID: "v", Surface: core.SurfaceLoadoutPanel, Kind: core.ViewEmbed,
			}}},
			"no html",
		},
		{
			"widget expression that does not parse",
			core.PluginManifest{APIVersion: 1, Views: []core.PluginView{{
				ID: "v", Surface: core.SurfaceLoadoutPanel, Kind: core.ViewWidget,
				Widget: &core.WidgetSpec{
					Type: core.WidgetPieChart, Source: core.SourceLoadoutEntries,
					Label: "item.category", Value: "sum(item.weight_g *",
				},
			}}},
			"unexpected",
		},
		{
			"widget reading a source its surface cannot provide",
			core.PluginManifest{APIVersion: 1, Views: []core.PluginView{{
				ID: "v", Surface: core.SurfaceItemTab, Kind: core.ViewWidget,
				Widget: &core.WidgetSpec{
					Type: core.WidgetPieChart, Source: core.SourceLoadoutEntries,
					Label: "item.category", Value: "item.weight_g",
				},
			}}},
			"cannot provide",
		},
		{
			"schema squatting the core namespace",
			core.PluginManifest{APIVersion: 1,
				Schemas: []core.PluginSchema{{Namespace: core.CoreNamespace, Definition: []byte(`{"type":"object"}`)}},
				Views: []core.PluginView{{
					ID: "v", Surface: core.SurfaceLoadoutPanel, Kind: core.ViewEmbed, HTML: "<p>",
				}}},
			"reserved by the platform",
		},
		{
			"schema that is not a schema",
			core.PluginManifest{APIVersion: 1,
				Schemas: []core.PluginSchema{{Namespace: "ul", Definition: []byte(`{"type": 42}`)}},
				Views: []core.PluginView{{
					ID: "v", Surface: core.SurfaceLoadoutPanel, Kind: core.ViewEmbed, HTML: "<p>",
				}}},
			"JSON Schema",
		},
		{
			"duplicate view ids",
			core.PluginManifest{APIVersion: 1, Views: []core.PluginView{
				{ID: "v", Surface: core.SurfaceLoadoutPanel, Kind: core.ViewEmbed, HTML: "<p>"},
				{ID: "v", Surface: core.SurfaceLoadoutPanel, Kind: core.ViewEmbed, HTML: "<p>"},
			}},
			"duplicate view id",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := h.plugins.Create(ctx, author.ID, PublishRequest{Name: "Broken", Manifest: tc.manifest})
			if !errors.Is(err, core.ErrInvalid) {
				t.Fatalf("error = %v, want ErrInvalid", err)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %q, want it to mention %q", err, tc.want)
			}
		})
	}
}

func TestPlugin_VersionsAreImmutableAndInstallsPin(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	author := h.profile(t, "author")
	user := h.profile(t, "user")

	published := h.publish(t, author, "Weight Breakdown", weightBreakdownManifest())

	install, err := h.plugins.Install(ctx, user.ID, InstallRequest{
		PluginID:    published.Plugin.ID,
		ScopeType:   core.ScopeProfile,
		ScopeID:     user.ID,
		GrantedCaps: []string{"loadout:read", "items:read"},
	})
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	if install.Install.Version != 1 {
		t.Fatalf("install pinned v%d, want v1", install.Install.Version)
	}

	// v2 changes the widget and asks for more access.
	next := weightBreakdownManifest()
	next.Views[0].Widget.Type = core.WidgetBarChart
	next.Capabilities.Storage = true
	if _, err := h.plugins.PublishVersion(ctx, author.ID, published.Plugin.ID, next, "now a bar chart"); err != nil {
		t.Fatalf("publish v2: %v", err)
	}

	// The pinned install is untouched: a publish cannot change approved behaviour.
	views, err := h.plugins.ListInstalls(ctx, core.ScopeProfile, user.ID)
	if err != nil {
		t.Fatalf("list installs: %v", err)
	}
	if len(views) != 1 {
		t.Fatalf("got %d installs, want 1", len(views))
	}
	view := views[0]
	if view.Install.Version != 1 {
		t.Errorf("install moved to v%d, want it pinned at v1", view.Install.Version)
	}
	if view.Manifest.Views[0].Widget.Type != core.WidgetPieChart {
		t.Errorf("install renders %q, want the v1 pie chart", view.Manifest.Views[0].Widget.Type)
	}
	if !view.UpgradeAvailable {
		t.Error("upgrade should be advertised")
	}
	// The new capability has not been granted, so upgrading requires a re-grant.
	if len(view.MissingCaps) != 1 || view.MissingCaps[0] != "storage:write" {
		t.Errorf("missing caps = %v, want [storage:write]", view.MissingCaps)
	}

	// v1 is still readable exactly as published.
	v1, err := h.plugins.Detail(ctx, published.Plugin.ID, 1)
	if err != nil {
		t.Fatalf("read v1: %v", err)
	}
	if v1.Version.Manifest.Capabilities.Storage {
		t.Error("v1 manifest was mutated by publishing v2")
	}
}

func TestPlugin_OnlyTheAuthorMayPublish(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	author := h.profile(t, "author")
	stranger := h.profile(t, "stranger")

	published := h.publish(t, author, "Weight Breakdown", weightBreakdownManifest())

	_, err := h.plugins.PublishVersion(ctx, stranger.ID, published.Plugin.ID, weightBreakdownManifest(), "hijack")
	if !errors.Is(err, core.ErrForbidden) {
		t.Errorf("stranger publish = %v, want ErrForbidden", err)
	}
	_, err = h.plugins.SetVisibility(ctx, stranger.ID, published.Plugin.ID, false)
	if !errors.Is(err, core.ErrForbidden) {
		t.Errorf("stranger unlist = %v, want ErrForbidden", err)
	}
}

func TestPlugin_CommunityOwnedPublishingRequiresAdmin(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	owner := h.profile(t, "owner")
	member := h.profile(t, "member")
	community, err := h.community.Create(ctx, owner.ID, core.Community{Name: "UL Backpacking"})
	if err != nil {
		t.Fatalf("create community: %v", err)
	}
	if _, err := h.community.Join(ctx, community.ID, member.ID); err != nil {
		t.Fatalf("join: %v", err)
	}

	req := PublishRequest{
		Name: "UL Score", OwnerType: core.OwnerCommunity, OwnerID: community.ID,
		Manifest: weightBreakdownManifest(), IsPublic: true,
	}
	if _, err := h.plugins.Create(ctx, member.ID, req); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("member publishing for a community = %v, want ErrForbidden", err)
	}
	if _, err := h.plugins.Create(ctx, owner.ID, req); err != nil {
		t.Errorf("admin publishing for a community failed: %v", err)
	}
}

func TestPlugin_SlugsAreUnique(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	author := h.profile(t, "author")

	h.publish(t, author, "Weight Breakdown", weightBreakdownManifest())
	_, err := h.plugins.Create(ctx, author.ID, PublishRequest{
		Name: "Weight Breakdown", Manifest: weightBreakdownManifest(),
	})
	if !errors.Is(err, core.ErrConflict) {
		t.Errorf("duplicate slug = %v, want ErrConflict", err)
	}
}

// --- Installing ---

func TestPlugin_InstallRequiresEveryRequestedCapability(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	author := h.profile(t, "author")
	user := h.profile(t, "user")
	published := h.publish(t, author, "Weight Breakdown", weightBreakdownManifest())

	// Granting a subset is refused rather than silently reduced, so the user always
	// sees the full ask before anything is enabled.
	_, err := h.plugins.Install(ctx, user.ID, InstallRequest{
		PluginID: published.Plugin.ID, ScopeType: core.ScopeProfile, ScopeID: user.ID,
		GrantedCaps: []string{"loadout:read"},
	})
	if !errors.Is(err, core.ErrInvalid) {
		t.Fatalf("partial grant = %v, want ErrInvalid", err)
	}
	if !strings.Contains(err.Error(), "items:read") {
		t.Errorf("error = %q, want it to name the missing capability", err)
	}
}

func TestPlugin_InstallScopeAuthorization(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	author := h.profile(t, "author")
	user := h.profile(t, "user")
	other := h.profile(t, "other")
	admin := h.profile(t, "admin")
	member := h.profile(t, "member")
	published := h.publish(t, author, "Weight Breakdown", weightBreakdownManifest())
	caps := []string{"loadout:read", "items:read"}

	community, err := h.community.Create(ctx, admin.ID, core.Community{Name: "Backpacking"})
	if err != nil {
		t.Fatalf("create community: %v", err)
	}
	if _, err := h.community.Join(ctx, community.ID, member.ID); err != nil {
		t.Fatalf("join: %v", err)
	}

	cases := []struct {
		name      string
		actor     string
		scopeType string
		scopeID   string
		wantErr   error
	}{
		{"own profile", user.ID, core.ScopeProfile, user.ID, nil},
		{"someone else's profile", user.ID, core.ScopeProfile, other.ID, core.ErrForbidden},
		{"community as admin", admin.ID, core.ScopeCommunity, community.ID, nil},
		{"community as plain member", member.ID, core.ScopeCommunity, community.ID, core.ErrForbidden},
		{"anonymous", "", core.ScopeProfile, user.ID, core.ErrForbidden},
		{"nonsense scope", user.ID, "loadout", user.ID, core.ErrInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := h.plugins.Install(ctx, tc.actor, InstallRequest{
				PluginID: published.Plugin.ID, ScopeType: tc.scopeType, ScopeID: tc.scopeID,
				GrantedCaps: caps,
			})
			if tc.wantErr == nil {
				if err != nil {
					t.Fatalf("install failed: %v", err)
				}
				return
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("error = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestPlugin_UnlistedPluginsAreAuthorOnly(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	author := h.profile(t, "author")
	user := h.profile(t, "user")

	private, err := h.plugins.Create(ctx, author.ID, PublishRequest{
		Name: "Work In Progress", Manifest: weightBreakdownManifest(), IsPublic: false,
	})
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	caps := []string{"loadout:read", "items:read"}

	// The author can install their own unlisted plugin, which is how you test one.
	if _, err := h.plugins.Install(ctx, author.ID, InstallRequest{
		PluginID: private.Plugin.ID, ScopeType: core.ScopeProfile, ScopeID: author.ID, GrantedCaps: caps,
	}); err != nil {
		t.Fatalf("author installing their own unlisted plugin failed: %v", err)
	}
	if _, err := h.plugins.Install(ctx, user.ID, InstallRequest{
		PluginID: private.Plugin.ID, ScopeType: core.ScopeProfile, ScopeID: user.ID, GrantedCaps: caps,
	}); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("stranger installing an unlisted plugin = %v, want ErrForbidden", err)
	}

	// Listing it makes it installable.
	if _, err := h.plugins.SetVisibility(ctx, author.ID, private.Plugin.ID, true); err != nil {
		t.Fatalf("list plugin: %v", err)
	}
	if _, err := h.plugins.Install(ctx, user.ID, InstallRequest{
		PluginID: private.Plugin.ID, ScopeType: core.ScopeProfile, ScopeID: user.ID, GrantedCaps: caps,
	}); err != nil {
		t.Errorf("install after listing failed: %v", err)
	}
}

func TestPlugin_ReinstallUpgradesInPlace(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	author := h.profile(t, "author")
	user := h.profile(t, "user")
	published := h.publish(t, author, "Weight Breakdown", weightBreakdownManifest())
	caps := []string{"loadout:read", "items:read"}

	first, err := h.plugins.Install(ctx, user.ID, InstallRequest{
		PluginID: published.Plugin.ID, ScopeType: core.ScopeProfile, ScopeID: user.ID, GrantedCaps: caps,
	})
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	if _, err := h.plugins.PublishVersion(ctx, author.ID, published.Plugin.ID, weightBreakdownManifest(), "v2"); err != nil {
		t.Fatalf("publish v2: %v", err)
	}
	second, err := h.plugins.Install(ctx, user.ID, InstallRequest{
		PluginID: published.Plugin.ID, ScopeType: core.ScopeProfile, ScopeID: user.ID, GrantedCaps: caps,
	})
	if err != nil {
		t.Fatalf("reinstall: %v", err)
	}

	if second.Install.ID != first.Install.ID {
		t.Errorf("reinstall created a new install %s, want it to upgrade %s in place", second.Install.ID, first.Install.ID)
	}
	if second.Install.Version != 2 {
		t.Errorf("reinstall pinned v%d, want v2", second.Install.Version)
	}
	installs, _ := h.plugins.ListInstalls(ctx, core.ScopeProfile, user.ID)
	if len(installs) != 1 {
		t.Errorf("got %d installs, want the reinstall to have replaced the first", len(installs))
	}
}

func TestPlugin_SettingsAreValidatedAndSecretsRedacted(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	author := h.profile(t, "author")
	user := h.profile(t, "user")
	published := h.publish(t, author, "Trip Route", mapManifest())
	caps := []string{"loadout:read", "storage:write", "network:maps.googleapis.com"}

	// The manifest marks the key required, so an install without one is refused.
	_, err := h.plugins.Install(ctx, user.ID, InstallRequest{
		PluginID: published.Plugin.ID, ScopeType: core.ScopeProfile, ScopeID: user.ID, GrantedCaps: caps,
	})
	if !errors.Is(err, core.ErrInvalid) {
		t.Fatalf("install without a required setting = %v, want ErrInvalid", err)
	}
	if !strings.Contains(err.Error(), "Maps API key") {
		t.Errorf("error = %q, want it to name the setting", err)
	}

	view, err := h.plugins.Install(ctx, user.ID, InstallRequest{
		PluginID: published.Plugin.ID, ScopeType: core.ScopeProfile, ScopeID: user.ID, GrantedCaps: caps,
		Settings: core.Metadata{"api_key": "sk-secret-value"},
	})
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	// The API response must not carry the secret back out.
	if got := view.Install.Settings["api_key"]; got == "sk-secret-value" {
		t.Error("the secret setting was returned verbatim, want it redacted")
	}
	// ...but it is stored intact, because the frame genuinely needs it.
	stored, err := h.store.GetPluginInstall(ctx, view.Install.ID)
	if err != nil {
		t.Fatalf("read install: %v", err)
	}
	if stored.Settings["api_key"] != "sk-secret-value" {
		t.Errorf("stored key = %v, want the real value", stored.Settings["api_key"])
	}
}

func TestPlugin_UninstallAndDisable(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	author := h.profile(t, "author")
	user := h.profile(t, "user")
	stranger := h.profile(t, "stranger")
	published := h.publish(t, author, "Weight Breakdown", weightBreakdownManifest())

	view, err := h.plugins.Install(ctx, user.ID, InstallRequest{
		PluginID: published.Plugin.ID, ScopeType: core.ScopeProfile, ScopeID: user.ID,
		GrantedCaps: []string{"loadout:read", "items:read"},
	})
	if err != nil {
		t.Fatalf("install: %v", err)
	}

	if err := h.plugins.Uninstall(ctx, stranger.ID, view.Install.ID); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("stranger uninstall = %v, want ErrForbidden", err)
	}
	disabled, err := h.plugins.SetEnabled(ctx, user.ID, view.Install.ID, false)
	if err != nil {
		t.Fatalf("disable: %v", err)
	}
	if disabled.Install.Enabled {
		t.Error("install still enabled after disabling")
	}
	if err := h.plugins.Uninstall(ctx, user.ID, view.Install.ID); err != nil {
		t.Fatalf("uninstall: %v", err)
	}
	installs, _ := h.plugins.ListInstalls(ctx, core.ScopeProfile, user.ID)
	if len(installs) != 0 {
		t.Errorf("got %d installs after uninstalling, want 0", len(installs))
	}
}

// --- Rendering ---

func TestPlugin_RenderWidgetAgainstARealLoadout(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	author := h.profile(t, "author")
	user := h.profile(t, "user")
	published := h.publish(t, author, "Weight Breakdown", weightBreakdownManifest())
	if _, err := h.plugins.Install(ctx, user.ID, InstallRequest{
		PluginID: published.Plugin.ID, ScopeType: core.ScopeProfile, ScopeID: user.ID,
		GrantedCaps: []string{"loadout:read", "items:read"},
	}); err != nil {
		t.Fatalf("install: %v", err)
	}
	loadout := h.loadoutFixture(t, user, "")

	views, err := h.plugins.RenderSurface(ctx, user.ID, RenderRequest{
		Surface: core.SurfaceLoadoutPanel, LoadoutID: loadout.Loadout.ID,
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if len(views) != 1 {
		t.Fatalf("got %d views, want 1", len(views))
	}

	view := views[0]
	if view.Error != "" {
		t.Fatalf("view reported an error: %s", view.Error)
	}
	if view.Kind != core.ViewWidget || view.Widget == nil {
		t.Fatalf("view = %+v, want an evaluated widget", view)
	}
	if len(view.Widget.Points) != 2 {
		t.Fatalf("got %d points, want shelter and sleep", len(view.Widget.Points))
	}
	// Sorted by value descending: sleep 560 g, then shelter 240 + 20.
	if view.Widget.Points[0].Label != "sleep" || view.Widget.Points[0].Value != 560 {
		t.Errorf("first point = %q/%v, want sleep/560", view.Widget.Points[0].Label, view.Widget.Points[0].Value)
	}
	if view.Widget.Points[1].Label != "shelter" || view.Widget.Points[1].Value != 260 {
		t.Errorf("second point = %q/%v, want shelter/260", view.Widget.Points[1].Label, view.Widget.Points[1].Value)
	}
	if view.Widget.Total != 820 {
		t.Errorf("total = %v, want 820", view.Widget.Total)
	}
}

func TestPlugin_RenderRespectsVisibility(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	author := h.profile(t, "author")
	user := h.profile(t, "user")
	snoop := h.profile(t, "snoop")
	published := h.publish(t, author, "Weight Breakdown", weightBreakdownManifest())
	if _, err := h.plugins.Install(ctx, snoop.ID, InstallRequest{
		PluginID: published.Plugin.ID, ScopeType: core.ScopeProfile, ScopeID: snoop.ID,
		GrantedCaps: []string{"loadout:read", "items:read"},
	}); err != nil {
		t.Fatalf("install: %v", err)
	}

	loadout := h.loadoutFixture(t, user, "")
	private := core.VisibilityPrivate
	if _, err := h.loadouts.Update(ctx, user.ID, loadout.Loadout.ID, UpdateLoadoutRequest{
		Visibility: &private,
	}); err != nil {
		t.Fatalf("make private: %v", err)
	}

	// A plugin must never become a way to read a loadout you could not open directly.
	_, err := h.plugins.RenderSurface(ctx, snoop.ID, RenderRequest{
		Surface: core.SurfaceLoadoutPanel, LoadoutID: loadout.Loadout.ID,
	})
	if !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("rendering a private loadout = %v, want ErrForbidden", err)
	}
}

func TestPlugin_UngrantedCapabilitiesStarveTheWidget(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	author := h.profile(t, "author")
	user := h.profile(t, "user")

	// A manifest that asks for nothing gets nothing: default-deny, enforced at render.
	manifest := weightBreakdownManifest()
	manifest.Capabilities = core.Capabilities{}
	published := h.publish(t, author, "Nosy", manifest)
	if _, err := h.plugins.Install(ctx, user.ID, InstallRequest{
		PluginID: published.Plugin.ID, ScopeType: core.ScopeProfile, ScopeID: user.ID,
	}); err != nil {
		t.Fatalf("install: %v", err)
	}
	loadout := h.loadoutFixture(t, user, "")

	views, err := h.plugins.RenderSurface(ctx, user.ID, RenderRequest{
		Surface: core.SurfaceLoadoutPanel, LoadoutID: loadout.Loadout.ID,
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if len(views) != 1 {
		t.Fatalf("got %d views, want 1", len(views))
	}
	if !views[0].Widget.Empty {
		t.Errorf("widget rendered %d points without items:read, want nothing", len(views[0].Widget.Points))
	}
}

func TestPlugin_RenderEmbedHandsOverAFrameAndContext(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	author := h.profile(t, "author")
	user := h.profile(t, "user")
	published := h.publish(t, author, "Trip Route", mapManifest())
	install, err := h.plugins.Install(ctx, user.ID, InstallRequest{
		PluginID: published.Plugin.ID, ScopeType: core.ScopeProfile, ScopeID: user.ID,
		GrantedCaps: []string{"loadout:read", "storage:write", "network:maps.googleapis.com"},
		Settings:    core.Metadata{"api_key": "sk-secret-value"},
	})
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	loadout := h.loadoutFixture(t, user, "")

	views, err := h.plugins.RenderSurface(ctx, user.ID, RenderRequest{
		Surface: core.SurfaceLoadoutSidebar, LoadoutID: loadout.Loadout.ID,
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if len(views) != 1 {
		t.Fatalf("got %d views, want 1", len(views))
	}
	view := views[0]

	if view.Kind != core.ViewEmbed {
		t.Fatalf("kind = %q, want embed", view.Kind)
	}
	if !strings.HasPrefix(view.EmbedURL, "https://sandbox.test/plugins/") {
		t.Errorf("embed url = %q, want it served from the sandbox origin", view.EmbedURL)
	}
	if !strings.Contains(view.EmbedURL, "install="+install.Install.ID) {
		t.Errorf("embed url = %q, want it to identify the install", view.EmbedURL)
	}
	if view.Height != 420 {
		t.Errorf("height = %d, want the manifest's 420", view.Height)
	}

	// The frame does get the real key: it is only useful in the browser, and the frame
	// is isolated by the sandbox.
	settings, _ := view.EmbedContext["settings"].(map[string]interface{})
	if settings["api_key"] != "sk-secret-value" {
		t.Errorf("frame settings = %v, want the real key", settings)
	}
	if _, ok := view.EmbedContext["loadout"]; !ok {
		t.Error("frame context has no loadout despite loadout:read")
	}
	// items:read was never requested, so the entry rows must not be there.
	if _, ok := view.EmbedContext["rows"]; ok {
		t.Error("frame context carries item rows without items:read")
	}
}

func TestPlugin_ABrokenViewDegradesToItsOwnCard(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	author := h.profile(t, "author")
	user := h.profile(t, "user")

	good := h.publish(t, author, "Weight Breakdown", weightBreakdownManifest())
	broken := h.publish(t, author, "Broken", weightBreakdownManifest())

	// Write a bad version straight to the store, bypassing publish validation, to
	// simulate a manifest that slipped through an older host version.
	bad := weightBreakdownManifest()
	bad.Views[0].Widget.Value = "sum(item.weight_g *"
	if err := h.store.CreatePluginVersion(ctx, core.PluginVersion{
		PluginID: broken.Plugin.ID, Version: 2, Manifest: bad,
	}); err != nil {
		t.Fatalf("seed a broken version: %v", err)
	}
	plugin, _ := h.store.GetPlugin(ctx, broken.Plugin.ID)
	plugin.LatestVersion = 2
	if err := h.store.UpdatePlugin(ctx, plugin); err != nil {
		t.Fatalf("bump latest: %v", err)
	}

	caps := []string{"loadout:read", "items:read"}
	for _, id := range []string{good.Plugin.ID, broken.Plugin.ID} {
		if _, err := h.plugins.Install(ctx, user.ID, InstallRequest{
			PluginID: id, ScopeType: core.ScopeProfile, ScopeID: user.ID, GrantedCaps: caps,
		}); err != nil {
			t.Fatalf("install %s: %v", id, err)
		}
	}
	loadout := h.loadoutFixture(t, user, "")

	views, err := h.plugins.RenderSurface(ctx, user.ID, RenderRequest{
		Surface: core.SurfaceLoadoutPanel, LoadoutID: loadout.Loadout.ID,
	})
	if err != nil {
		t.Fatalf("one broken plugin failed the whole render: %v", err)
	}
	if len(views) != 2 {
		t.Fatalf("got %d views, want both the working and the broken one", len(views))
	}

	var errored, worked int
	for _, v := range views {
		if v.Error != "" {
			errored++
			continue
		}
		if v.Widget != nil && len(v.Widget.Points) == 2 {
			worked++
		}
	}
	if errored != 1 || worked != 1 {
		t.Errorf("got %d errored and %d working views, want exactly one of each", errored, worked)
	}
}

func TestPlugin_CommunityInstallsRenderForVisitors(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	author := h.profile(t, "author")
	admin := h.profile(t, "admin")
	visitor := h.profile(t, "visitor")

	community, err := h.community.Create(ctx, admin.ID, core.Community{Name: "UL Backpacking"})
	if err != nil {
		t.Fatalf("create community: %v", err)
	}
	published := h.publish(t, author, "Weight Breakdown", weightBreakdownManifest())
	if _, err := h.plugins.Install(ctx, admin.ID, InstallRequest{
		PluginID: published.Plugin.ID, ScopeType: core.ScopeCommunity, ScopeID: community.ID,
		GrantedCaps: []string{"loadout:read", "items:read"},
	}); err != nil {
		t.Fatalf("community install: %v", err)
	}

	// A loadout posted to the community renders the community's plugins for anyone who
	// can see it, even a visitor who installed nothing.
	loadout := h.loadoutFixture(t, admin, community.ID)
	views, err := h.plugins.RenderSurface(ctx, visitor.ID, RenderRequest{
		Surface: core.SurfaceLoadoutPanel, LoadoutID: loadout.Loadout.ID,
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if len(views) != 1 {
		t.Fatalf("got %d views, want the community's plugin to render", len(views))
	}
	if views[0].Widget == nil || len(views[0].Widget.Points) != 2 {
		t.Errorf("community plugin did not render data: %+v", views[0])
	}
}

// --- Plugin storage ---

func TestPlugin_StorageIsNamespacedAndGated(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	author := h.profile(t, "author")
	user := h.profile(t, "user")
	stranger := h.profile(t, "stranger")

	published := h.publish(t, author, "Trip Route", mapManifest())
	if _, err := h.plugins.Install(ctx, user.ID, InstallRequest{
		PluginID: published.Plugin.ID, ScopeType: core.ScopeProfile, ScopeID: user.ID,
		GrantedCaps: []string{"loadout:read", "storage:write", "network:maps.googleapis.com"},
		Settings:    core.Metadata{"api_key": "sk"},
	}); err != nil {
		t.Fatalf("install: %v", err)
	}
	loadout := h.loadoutFixture(t, user, "")
	gpx := core.Metadata{"track": "50.1,-120.2;50.3,-120.4"}

	// The owner may store against their own loadout.
	if _, err := h.plugins.PutDatum(ctx, user.ID, published.Plugin.ID, core.ScopeLoadout, loadout.Loadout.ID, "route", gpx); err != nil {
		t.Fatalf("owner write: %v", err)
	}
	data, err := h.plugins.ListData(ctx, user.ID, published.Plugin.ID, core.ScopeLoadout, loadout.Loadout.ID)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if len(data) != 1 || data[0].Key != "route" {
		t.Fatalf("stored data = %+v, want one route key", data)
	}

	// A stranger who has not installed the plugin cannot write through it...
	if _, err := h.plugins.PutDatum(ctx, stranger.ID, published.Plugin.ID, core.ScopeLoadout, loadout.Loadout.ID, "route", gpx); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("uninstalled write = %v, want ErrForbidden", err)
	}
	// ...and neither can one who has, because they do not own the loadout.
	if _, err := h.plugins.Install(ctx, stranger.ID, InstallRequest{
		PluginID: published.Plugin.ID, ScopeType: core.ScopeProfile, ScopeID: stranger.ID,
		GrantedCaps: []string{"loadout:read", "storage:write", "network:maps.googleapis.com"},
		Settings:    core.Metadata{"api_key": "sk"},
	}); err != nil {
		t.Fatalf("stranger install: %v", err)
	}
	if _, err := h.plugins.PutDatum(ctx, stranger.ID, published.Plugin.ID, core.ScopeLoadout, loadout.Loadout.ID, "route", gpx); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("write to someone else's loadout = %v, want ErrForbidden", err)
	}

	// Item scope is read-only until there is moderation for it.
	if _, err := h.plugins.PutDatum(ctx, user.ID, published.Plugin.ID, core.ScopeItem, "tarp", "note", gpx); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("item-scoped write = %v, want ErrForbidden", err)
	}

	// Oversized values are refused rather than accepted and truncated.
	huge := core.Metadata{"blob": strings.Repeat("x", maxPluginDatumBytes+1)}
	if _, err := h.plugins.PutDatum(ctx, user.ID, published.Plugin.ID, core.ScopeLoadout, loadout.Loadout.ID, "blob", huge); !errors.Is(err, core.ErrInvalid) {
		t.Errorf("oversized write = %v, want ErrInvalid", err)
	}
}

func TestPlugin_StoredDataReachesTheRenderContext(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	author := h.profile(t, "author")
	user := h.profile(t, "user")

	published := h.publish(t, author, "Trip Route", mapManifest())
	if _, err := h.plugins.Install(ctx, user.ID, InstallRequest{
		PluginID: published.Plugin.ID, ScopeType: core.ScopeProfile, ScopeID: user.ID,
		GrantedCaps: []string{"loadout:read", "storage:write", "network:maps.googleapis.com"},
		Settings:    core.Metadata{"api_key": "sk"},
	}); err != nil {
		t.Fatalf("install: %v", err)
	}
	loadout := h.loadoutFixture(t, user, "")
	if _, err := h.plugins.PutDatum(ctx, user.ID, published.Plugin.ID, core.ScopeLoadout, loadout.Loadout.ID,
		"route", core.Metadata{"track": "50.1,-120.2"}); err != nil {
		t.Fatalf("store: %v", err)
	}

	views, err := h.plugins.RenderSurface(ctx, user.ID, RenderRequest{
		Surface: core.SurfaceLoadoutSidebar, LoadoutID: loadout.Loadout.ID,
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	// A route plugin should not have to fetch its own saved track after rendering.
	data, _ := views[0].EmbedContext["data"].(map[string]interface{})
	route, _ := data["route"].(map[string]interface{})
	if route["track"] != "50.1,-120.2" {
		t.Errorf("render context data = %+v, want the saved track", data)
	}
}

package seed

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/gmccloskey/loadouts/backend/internal/core"
	"github.com/gmccloskey/loadouts/backend/internal/service"
)

// The three demo plugins exist to prove the model is general rather than to be useful.
// Between them they exercise every axis the design has:
//
//	weight-breakdown  widget · pie chart  · reads the loadout   · no settings
//	ul-score          widget · table      · reads a community layer and declares a schema
//	trip-route        embed  · iframe     · network + secret setting + plugin storage
//
// If a fourth kind of plugin cannot be expressed as a variation on one of these, the
// manifest is missing something.
func seedPlugins(ctx context.Context, s Services, authorID, communityID string) error {
	if s.Plugins == nil {
		return nil
	}

	breakdown, err := s.Plugins.Create(ctx, authorID, service.PublishRequest{
		Slug:        "weight-breakdown",
		Name:        "Weight Breakdown",
		Description: "Where your pack weight actually goes, by category.",
		OwnerType:   core.OwnerProfile,
		OwnerID:     authorID,
		IsPublic:    true,
		Changelog:   "Initial release.",
		Manifest:    weightBreakdownManifest(),
	})
	if err != nil {
		return fmt.Errorf("seed weight-breakdown plugin: %w", err)
	}

	ulScore, err := s.Plugins.Create(ctx, authorID, service.PublishRequest{
		Slug:        "ul-score",
		Name:        "UL Score Table",
		Description: "Ranks your kit by the community's ultralight score, and flags the worst offender.",
		OwnerType:   core.OwnerCommunity,
		OwnerID:     communityID,
		IsPublic:    true,
		Changelog:   "Initial release.",
		Manifest:    ulScoreManifest(),
	})
	if err != nil {
		return fmt.Errorf("seed ul-score plugin: %w", err)
	}

	route, err := s.Plugins.Create(ctx, authorID, service.PublishRequest{
		Slug:        "trip-route",
		Name:        "Trip Route Map",
		Description: "Pin the route this loadout was packed for. Saves the track on the loadout itself.",
		OwnerType:   core.OwnerProfile,
		OwnerID:     authorID,
		IsPublic:    true,
		Changelog:   "Initial release.",
		Manifest:    tripRouteManifest(),
	})
	if err != nil {
		return fmt.Errorf("seed trip-route plugin: %w", err)
	}

	// Install the two widgets so a fresh boot shows plugins doing something rather than
	// an empty directory. The embed is left uninstalled on purpose: it needs a Maps API
	// key, and an install that renders a key error on first load teaches the wrong thing.
	installs := []service.InstallRequest{
		{
			PluginID:    breakdown.Plugin.ID,
			ScopeType:   core.ScopeProfile,
			ScopeID:     authorID,
			GrantedCaps: breakdown.Version.Manifest.Capabilities.List(),
		},
		{
			PluginID:    ulScore.Plugin.ID,
			ScopeType:   core.ScopeCommunity,
			ScopeID:     communityID,
			GrantedCaps: ulScore.Version.Manifest.Capabilities.List(),
		},
	}
	for _, req := range installs {
		if _, err := s.Plugins.Install(ctx, authorID, req); err != nil {
			return fmt.Errorf("seed install %s: %w", req.PluginID, err)
		}
	}

	_ = route // published to the directory, installed by hand.
	return nil
}

// weightBreakdownManifest is the simplest useful plugin: one pie chart, no configuration,
// and only the two read capabilities it genuinely needs.
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
				Type:      core.WidgetPieChart,
				Source:    core.SourceLoadoutEntries,
				GroupBy:   "item.category",
				Label:     "item.category",
				Value:     "sum(item.weight_g * entry.quantity)",
				Format:    core.FormatGrams,
				SortBy:    "value",
				SortDesc:  true,
				EmptyText: "Add some gear and the breakdown appears here.",
			},
		}},
	}
}

// ulScoreManifest shows a plugin reaching into a community metadata layer, which is the
// case that justifies plugins existing at all: the host has no idea what an "ul_score" is,
// the community defined it, and a plugin makes it legible.
//
// It declares a schema for its namespace so the values it reads have a contract, and pairs
// the table with a stat grid to show two views shipping in one plugin.
func ulScoreManifest() core.PluginManifest {
	return core.PluginManifest{
		APIVersion:   core.ManifestAPIVersion,
		Capabilities: core.Capabilities{ReadLoadout: true, ReadItems: true, ReadCommunity: true},
		Schemas: []core.PluginSchema{{
			Namespace:  "ul_backpacking",
			Definition: json.RawMessage(ulScoreSchema),
		}},
		Views: []core.PluginView{
			{
				ID:      "scores",
				Title:   "UL scores",
				Surface: core.SurfaceLoadoutPanel,
				Kind:    core.ViewWidget,
				Widget: &core.WidgetSpec{
					Type:   core.WidgetTable,
					Source: core.SourceLoadoutEntries,
					// Items the community has not scored would be all-blank rows.
					Filter: "item.metadata.ul_backpacking.ul_score != null",
					Columns: []core.WidgetColumn{
						{Label: "Item", Value: "item.name", Format: core.FormatText},
						{Label: "UL", Value: "item.metadata.ul_backpacking.ul_score", Format: core.FormatNumber},
						{Label: "Comfort", Value: "item.metadata.ul_backpacking.comfort", Format: core.FormatNumber},
						{Label: "Durability", Value: "item.metadata.ul_backpacking.durability", Format: core.FormatNumber},
						{Label: "Weight", Value: "item.weight_g * entry.quantity", Format: core.FormatGrams},
						// percent() scales to 0-100, which is what FormatPercent expects; a
						// bare ratio would render as "0.4%" instead of "36.3%".
						{Label: "Share", Value: "percent(item.weight_g * entry.quantity, sum(item.weight_g * entry.quantity))", Format: core.FormatPercent},
					},
					SortBy:    "item.metadata.ul_backpacking.ul_score",
					Limit:     25,
					EmptyText: "Nothing in this loadout has been scored by the community yet.",
				},
			},
			{
				ID:      "summary",
				Title:   "Kit summary",
				Surface: core.SurfaceLoadoutSidebar,
				Kind:    core.ViewWidget,
				Widget: &core.WidgetSpec{
					Type:   core.WidgetStatGrid,
					Source: core.SourceLoadoutEntries,
					Stats: []core.WidgetStat{
						{
							Label:  "Mean UL score",
							Value:  "avg(item.metadata.ul_backpacking.ul_score)",
							Format: core.FormatNumber,
							Help:   "Across scored items only.",
						},
						{
							Label:  "Heaviest item",
							Value:  "max(item.weight_g * entry.quantity)",
							Format: core.FormatGrams,
						},
						{
							Label:  "Consumables",
							Value:  "sum(if(item.consumable, item.weight_g * entry.quantity, 0))",
							Format: core.FormatGrams,
							Help:   "Weight you eat or burn off along the way.",
						},
					},
					EmptyText: "No gear yet.",
				},
			},
		},
	}
}

// ulScoreSchema is the contract for the namespace the plugin reads. Declaring it is what
// lets a community adopt the plugin and get validation on the fields it depends on.
const ulScoreSchema = `{
  "type": "object",
  "properties": {
    "ul_score":   {"type": "number", "minimum": 0, "maximum": 10},
    "comfort":    {"type": "number", "minimum": 0, "maximum": 10},
    "durability": {"type": "number", "minimum": 0, "maximum": 10}
  },
  "additionalProperties": false
}`

// tripRouteManifest is the powerful tier: the author's own HTML and JavaScript, running in
// a sandboxed frame.
//
// It is the example that justifies the whole embed design, because it needs three things a
// declarative widget can never have: a third-party script (Maps), a per-install secret (the
// API key), and somewhere to put data the host has no schema for (the route). It gets all
// three without the host ever executing author code on its own origin.
func tripRouteManifest() core.PluginManifest {
	return core.PluginManifest{
		APIVersion: core.ManifestAPIVersion,
		Capabilities: core.Capabilities{
			ReadLoadout: true,
			Storage:     true,
			Network:     []string{"maps.googleapis.com"},
		},
		Settings: []core.SettingDefinition{{
			Key:      "api_key",
			Label:    "Google Maps API key",
			Type:     core.SettingSecret,
			Required: true,
			Help:     "Your own key. It is sent only to the map frame, never rendered on the page.",
		}},
		Views: []core.PluginView{{
			ID:      "route",
			Title:   "Trip route",
			Surface: core.SurfaceLoadoutSidebar,
			Kind:    core.ViewEmbed,
			Height:  360,
			HTML:    tripRouteHTML,
		}},
	}
}

// tripRouteHTML is the frame document.
//
// Note what it never does: it holds no credentials and makes no API call. It cannot — the
// frame is sandboxed without allow-same-origin, so it runs on an opaque origin with no
// cookies. Every privileged operation goes through window.Loadouts, which postMessages the
// parent, and the parent is the authenticated party. An author gets a familiar-looking SDK
// and the host keeps the only key to the door.
const tripRouteHTML = `<!doctype html>
<meta charset="utf-8">
<style>
  #map { height: 100%; width: 100%; border-radius: 8px; background: #1c1917; }
  #hint { font: 12px/1.5 ui-sans-serif, system-ui; color: #a8a29e; padding: 8px 2px; }
  #hint b { color: #e7e5e4; font-weight: 600; }
  .err { color: #fca5a5; }
</style>
<div id="map"></div>
<div id="hint">Loading route&hellip;</div>
<script>
  const hint = document.getElementById('hint');

  Loadouts.ready(async (ctx) => {
    const key = ctx.settings.api_key;
    if (!key) {
      hint.className = 'err';
      hint.textContent = 'Add a Maps API key in this plugin\'s settings to see the route.';
      return;
    }

    // The plugin's own saved data comes back on the context, scoped to this loadout.
    const saved = ctx.data.route;
    hint.innerHTML = saved
      ? 'Route for <b>' + ctx.loadout.name + '</b>: ' + saved.name
      : 'No route saved for <b>' + ctx.loadout.name + '</b> yet.';

    await loadMaps(key);
    const map = new google.maps.Map(document.getElementById('map'), {
      center: saved ? saved.path[0] : { lat: 37.87, lng: -119.38 },
      zoom: saved ? 11 : 8,
      disableDefaultUI: true,
    });

    if (saved) {
      new google.maps.Polyline({
        map, path: saved.path, strokeColor: '#fb923c', strokeWeight: 3,
      });
    }

    // Clicking builds a track, which is persisted against the loadout under this
    // plugin's namespace. Nothing else can read or write that key.
    const path = saved ? saved.path.slice() : [];
    map.addListener('click', async (e) => {
      path.push({ lat: e.latLng.lat(), lng: e.latLng.lng() });
      new google.maps.Polyline({ map, path, strokeColor: '#fb923c', strokeWeight: 3 });
      await Loadouts.save('route', { name: saved ? saved.name : 'Untitled route', path });
      hint.textContent = path.length + ' points saved.';
    });
  });

  function loadMaps(key) {
    return new Promise((resolve, reject) => {
      const s = document.createElement('script');
      s.src = 'https://maps.googleapis.com/maps/api/js?key=' + encodeURIComponent(key);
      s.onload = resolve;
      s.onerror = () => reject(new Error('Google Maps failed to load.'));
      document.head.appendChild(s);
    });
  }
</script>`

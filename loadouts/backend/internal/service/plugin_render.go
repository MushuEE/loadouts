package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/gmccloskey/loadouts/backend/internal/core"
)

// Rendering installed plugin views, and the namespaced storage they write to.
//
// The host builds every byte of data a plugin sees. A widget is evaluated here and leaves
// as literal values; an embed receives a context object and nothing else. Neither ever
// gets a database handle, a session, or a way to ask for data it was not granted.

// Render limits.
const (
	// maxCommunityRows bounds a community catalog widget. A community can hold tens of
	// thousands of items and no chart is improved by all of them.
	maxCommunityRows = 500
	// maxPluginDatumBytes keeps plugin storage a place for state, not a free blob store.
	maxPluginDatumBytes = 64 * 1024
	maxPluginDatumKey   = 128
)

// RenderRequest asks for everything installed on one surface of one thing.
type RenderRequest struct {
	Surface core.Surface
	// Exactly one of these is meaningful, selected by the surface's scope type.
	LoadoutID   string
	ItemID      string
	CommunityID string
}

// RenderSurface evaluates every enabled view installed on a surface.
//
// A plugin that fails to render produces a RenderedView carrying an Error rather than
// failing the request, so one broken plugin degrades to a message in its own card instead
// of taking down the page it lives on.
func (s *PluginService) RenderSurface(ctx context.Context, viewerProfileID string, req RenderRequest) ([]core.RenderedView, error) {
	if !req.Surface.IsValid() {
		return nil, fmt.Errorf("%w: unknown surface %q", core.ErrInvalid, req.Surface)
	}

	scopeID, communityID, data, err := s.buildScope(ctx, viewerProfileID, req)
	if err != nil {
		return nil, err
	}

	installs, err := s.installsForSurface(ctx, viewerProfileID, req.Surface, communityID)
	if err != nil {
		return nil, err
	}

	rendered := []core.RenderedView{}
	for _, install := range installs {
		if !install.Enabled {
			continue
		}
		pv, err := s.store.GetPluginVersion(ctx, install.PluginID, install.Version)
		if err != nil {
			continue
		}
		caps := effectiveCaps(pv.Manifest.Capabilities, install)

		for _, view := range pv.Manifest.Views {
			if view.Surface != req.Surface {
				continue
			}
			rendered = append(rendered, s.renderView(ctx, viewerProfileID, install, pv, view, caps, data, req.Surface.ScopeType(), scopeID))
		}
	}

	// Deterministic order: two page loads should not shuffle the panels.
	sort.SliceStable(rendered, func(a, b int) bool {
		if rendered[a].Title != rendered[b].Title {
			return rendered[a].Title < rendered[b].Title
		}
		return rendered[a].ViewID < rendered[b].ViewID
	})
	return rendered, nil
}

// buildScope resolves the thing being looked at and turns it into widget data. It returns
// the scope ID, the community context, and the data.
func (s *PluginService) buildScope(ctx context.Context, viewerProfileID string, req RenderRequest) (string, string, core.WidgetData, error) {
	switch req.Surface.ScopeType() {
	case core.ScopeLoadout:
		if req.LoadoutID == "" {
			return "", "", core.WidgetData{}, fmt.Errorf("%w: a loadout id is required for %q", core.ErrInvalid, req.Surface)
		}
		// Detail enforces visibility, so a plugin can never be used to read a loadout
		// its viewer could not open directly.
		detail, err := s.loadouts.Detail(ctx, req.LoadoutID, viewerProfileID)
		if err != nil {
			return "", "", core.WidgetData{}, err
		}
		return detail.Loadout.ID, detail.Loadout.CommunityID, loadoutWidgetData(detail, viewerProfileID), nil

	case core.ScopeItem:
		if req.ItemID == "" {
			return "", "", core.WidgetData{}, fmt.Errorf("%w: an item id is required for %q", core.ErrInvalid, req.Surface)
		}
		lctx := core.LayerContext{ViewerProfileID: viewerProfileID, CommunityID: req.CommunityID}
		item, err := s.items.ResolveItem(ctx, req.ItemID, lctx)
		if err != nil {
			return "", "", core.WidgetData{}, err
		}
		row := map[string]interface{}{"item": itemMap(item)}
		return item.ID, req.CommunityID, core.WidgetData{
			Rows:    []map[string]interface{}{row},
			Context: map[string]interface{}{"item": itemMap(item), "viewer": viewerMap(viewerProfileID)},
		}, nil

	case core.ScopeCommunity:
		if req.CommunityID == "" {
			return "", "", core.WidgetData{}, fmt.Errorf("%w: a community id is required for %q", core.ErrInvalid, req.Surface)
		}
		community, err := s.community.Get(ctx, req.CommunityID)
		if err != nil {
			return "", "", core.WidgetData{}, err
		}
		data, err := s.communityWidgetData(ctx, community, viewerProfileID)
		if err != nil {
			return "", "", core.WidgetData{}, err
		}
		return community.ID, community.ID, data, nil
	}
	return "", "", core.WidgetData{}, fmt.Errorf("%w: surface %q has no scope", core.ErrInvalid, req.Surface)
}

// installsForSurface collects the installs that may contribute to a surface.
//
// A viewer sees their own installs plus whatever the relevant community installed. Where
// both installed the same plugin the viewer's own install wins, since it carries their
// grants and their settings.
func (s *PluginService) installsForSurface(ctx context.Context, viewerProfileID string, surface core.Surface, communityID string) ([]core.PluginInstall, error) {
	byPlugin := map[string]core.PluginInstall{}
	order := []string{}

	add := func(installs []core.PluginInstall) {
		for _, i := range installs {
			if _, seen := byPlugin[i.PluginID]; seen {
				continue
			}
			byPlugin[i.PluginID] = i
			order = append(order, i.PluginID)
		}
	}

	// A community tab shows what the community installed, not what the visitor did.
	if surface != core.SurfaceCommunityTab && viewerProfileID != "" {
		mine, err := s.store.ListPluginInstalls(ctx, core.ScopeProfile, viewerProfileID)
		if err != nil {
			return nil, err
		}
		add(mine)
	}
	if communityID != "" {
		theirs, err := s.store.ListPluginInstalls(ctx, core.ScopeCommunity, communityID)
		if err != nil {
			return nil, err
		}
		add(theirs)
	}

	out := make([]core.PluginInstall, 0, len(order))
	for _, id := range order {
		out = append(out, byPlugin[id])
	}
	return out, nil
}

// renderView produces one card. Errors are captured onto the view, never returned.
func (s *PluginService) renderView(
	ctx context.Context,
	viewerProfileID string,
	install core.PluginInstall,
	pv core.PluginVersion,
	view core.PluginView,
	caps core.Capabilities,
	data core.WidgetData,
	scopeType, scopeID string,
) core.RenderedView {
	out := core.RenderedView{
		InstallID: install.ID,
		PluginID:  install.PluginID,
		Version:   install.Version,
		ViewID:    view.ID,
		Title:     view.Title,
		Surface:   view.Surface,
		Kind:      view.Kind,
		Height:    view.Height,
	}

	// Stored data is part of the render context: a calculator that remembers its inputs
	// should show them without a second round trip.
	stored := map[string]interface{}{}
	if caps.Storage {
		if data, err := s.store.ListPluginData(ctx, install.PluginID, scopeType, scopeID); err == nil {
			for _, d := range data {
				stored[d.Key] = map[string]interface{}(d.Value)
			}
		}
	}

	switch view.Kind {
	case core.ViewWidget:
		if view.Widget == nil {
			out.Error = "this view declares no widget"
			return out
		}
		scoped := applyCapabilities(data, caps)
		scoped.Context = withRenderContext(scoped.Context, map[string]interface{}{
			// Widget output is rendered to the page, so settings arrive redacted: a
			// widget must not be able to print an API key onto someone's screen.
			"settings": map[string]interface{}(redactSecrets(pv.Manifest.Settings, install.Settings)),
			"data":     stored,
			"viewer":   viewerMap(viewerProfileID),
		})

		render, err := core.EvaluateWidget(*view.Widget, scoped)
		if err != nil {
			out.Error = err.Error()
			return out
		}
		out.Widget = &render

	case core.ViewEmbed:
		out.EmbedURL = s.embedURL(install, view.ID)
		// The frame gets real settings. A Maps key is only useful in the browser, and
		// the frame is already isolated by the sandbox.
		out.EmbedContext = map[string]interface{}{
			"api_version":  core.ManifestAPIVersion,
			"install_id":   install.ID,
			"plugin_id":    install.PluginID,
			"version":      install.Version,
			"view_id":      view.ID,
			"scope":        map[string]interface{}{"type": scopeType, "id": scopeID},
			"capabilities": caps.List(),
			"settings":     map[string]interface{}(install.Settings),
			"data":         stored,
			"viewer":       viewerMap(viewerProfileID),
		}
		scoped := applyCapabilities(data, caps)
		for k, v := range scoped.Context {
			out.EmbedContext[k] = v
		}
		if caps.ReadItems {
			out.EmbedContext["rows"] = scoped.Rows
		}

	default:
		out.Error = fmt.Sprintf("unknown view kind %q", view.Kind)
	}
	return out
}

func (s *PluginService) embedURL(install core.PluginInstall, viewID string) string {
	return fmt.Sprintf("%s/plugins/%s/versions/%d/views/%s/frame?install=%s",
		strings.TrimSuffix(s.sandboxBase, "/"),
		url.PathEscape(install.PluginID),
		install.Version,
		url.PathEscape(viewID),
		url.QueryEscape(install.ID),
	)
}

// applyCapabilities strips anything the install was not granted. This is the enforcement
// point for default-deny: a plugin that asked for nothing renders against nothing.
func applyCapabilities(data core.WidgetData, caps core.Capabilities) core.WidgetData {
	out := core.WidgetData{Context: map[string]interface{}{}}
	if caps.ReadItems {
		out.Rows = data.Rows
	}
	for k, v := range data.Context {
		switch k {
		case "loadout":
			if caps.ReadLoadout {
				out.Context[k] = v
			}
		case "item":
			if caps.ReadItems {
				out.Context[k] = v
			}
		case "community":
			if caps.ReadCommunity {
				out.Context[k] = v
			}
		default:
			out.Context[k] = v
		}
	}
	return out
}

func withRenderContext(base, extra map[string]interface{}) map[string]interface{} {
	out := map[string]interface{}{}
	for k, v := range base {
		out[k] = v
	}
	for k, v := range extra {
		out[k] = v
	}
	return out
}

// --- Data shaping ---

// loadoutWidgetData flattens a resolved loadout into rows an expression can walk.
func loadoutWidgetData(detail core.LoadoutDetail, viewerProfileID string) core.WidgetData {
	rows := []map[string]interface{}{}
	var walk func(entries []core.ResolvedEntry, depth int)
	walk = func(entries []core.ResolvedEntry, depth int) {
		for _, e := range entries {
			rows = append(rows, map[string]interface{}{
				"entry": map[string]interface{}{
					"id":              e.Entry.ID,
					"slot_id":         e.Entry.SlotID,
					"parent_entry_id": e.Entry.ParentEntryID,
					"quantity":        float64(e.Entry.Quantity),
					"note":            e.Entry.Note,
					"position":        float64(e.Entry.Position),
					"depth":           float64(depth),
				},
				"item": itemMap(e.Item),
			})
			walk(e.Children, depth+1)
		}
	}
	walk(detail.Entries, 0)

	l := detail.Loadout
	return core.WidgetData{
		Rows: rows,
		Context: map[string]interface{}{
			"loadout": map[string]interface{}{
				"id":               l.ID,
				"name":             l.Name,
				"description":      l.Description,
				"owner_profile_id": l.OwnerProfileID,
				"owner_handle":     detail.Owner.Handle,
				"community_id":     l.CommunityID,
				"template_id":      l.TemplateID,
				"template_name":    detail.Template.Template.Name,
				"template_version": float64(l.TemplateVersion),
				"visibility":       string(l.Visibility),
				"status":           string(l.Status),
				"stats": map[string]interface{}{
					"total_weight_g":      detail.Stats.TotalWeightG,
					"base_weight_g":       detail.Stats.BaseWeightG,
					"consumable_weight_g": detail.Stats.ConsumableWeightG,
					"total_cost_cents":    detail.Stats.TotalCostCents,
					"item_count":          float64(detail.Stats.ItemCount),
				},
			},
			"viewer": viewerMap(viewerProfileID),
		},
	}
}

// communityWidgetData builds rows from a community's catalog, resolved through that
// community's layer so a widget sees the community's view of each item.
func (s *PluginService) communityWidgetData(ctx context.Context, community core.Community, viewerProfileID string) (core.WidgetData, error) {
	items, err := s.store.ListItems(ctx, "")
	if err != nil {
		return core.WidgetData{}, err
	}
	if len(items) > maxCommunityRows {
		items = items[:maxCommunityRows]
	}

	lctx := core.LayerContext{ViewerProfileID: viewerProfileID, CommunityID: community.ID}
	ids := make([]string, 0, len(items))
	for _, i := range items {
		ids = append(ids, i.ID)
	}
	resolved := s.items.ResolveItems(ctx, ids, lctx)

	rows := make([]map[string]interface{}, 0, len(ids))
	for _, id := range ids {
		item, ok := resolved[id]
		if !ok {
			continue
		}
		rows = append(rows, map[string]interface{}{"item": itemMap(item)})
	}

	return core.WidgetData{
		Rows: rows,
		Context: map[string]interface{}{
			"community": map[string]interface{}{
				"id":           community.ID,
				"slug":         community.Slug,
				"name":         community.Name,
				"member_count": float64(community.MemberCount),
				"item_count":   float64(len(rows)),
			},
			"viewer": viewerMap(viewerProfileID),
		},
	}, nil
}

// itemMap flattens a resolved item. The core numbers are hoisted to the top level so the
// common expression reads `item.weight_g * entry.quantity` rather than reaching four
// levels into the metadata map; both forms work.
func itemMap(item core.ResolvedItem) map[string]interface{} {
	return map[string]interface{}{
		"id":         item.ID,
		"name":       item.Name,
		"category":   item.Category,
		"image_url":  item.ImageURL,
		"origin":     item.Origin,
		"verified":   item.Verified,
		"weight_g":   item.WeightG(),
		"cost_cents": item.CostCents(),
		"consumable": item.IsConsumable(),
		"metadata":   item.Metadata,
	}
}

func viewerMap(profileID string) map[string]interface{} {
	return map[string]interface{}{
		"profile_id":       profileID,
		"is_authenticated": profileID != "",
	}
}

// effectiveCaps intersects what a manifest asks for with what the install granted. The
// manifest can only ever narrow: asking for more after an upgrade does nothing until the
// installer re-grants.
func effectiveCaps(requested core.Capabilities, install core.PluginInstall) core.Capabilities {
	out := core.Capabilities{}
	if requested.ReadLoadout && install.GrantedCaps.Contains("loadout:read") {
		out.ReadLoadout = true
	}
	if requested.ReadItems && install.GrantedCaps.Contains("items:read") {
		out.ReadItems = true
	}
	if requested.ReadCommunity && install.GrantedCaps.Contains("community:read") {
		out.ReadCommunity = true
	}
	if requested.Storage && install.GrantedCaps.Contains("storage:write") {
		out.Storage = true
	}
	for _, host := range requested.Network {
		if install.GrantedCaps.Contains("network:" + host) {
			out.Network = append(out.Network, host)
		}
	}
	return out
}

// --- Plugin storage ---

// PutDatum writes one plugin-namespaced value.
func (s *PluginService) PutDatum(ctx context.Context, actorProfileID, pluginID, scopeType, scopeID, dataKey string, value core.Metadata) (core.PluginDatum, error) {
	if strings.TrimSpace(dataKey) == "" {
		return core.PluginDatum{}, fmt.Errorf("%w: a storage key is required", core.ErrInvalid)
	}
	if len(dataKey) > maxPluginDatumKey {
		return core.PluginDatum{}, fmt.Errorf("%w: storage keys are limited to %d characters", core.ErrInvalid, maxPluginDatumKey)
	}
	if size := jsonSize(value); size > maxPluginDatumBytes {
		return core.PluginDatum{}, fmt.Errorf("%w: %d bytes exceeds the %d byte limit for one key",
			core.ErrInvalid, size, maxPluginDatumBytes)
	}
	if err := s.requireStorageAccess(ctx, actorProfileID, pluginID, scopeType, scopeID, true); err != nil {
		return core.PluginDatum{}, err
	}

	datum := core.PluginDatum{
		PluginID:  pluginID,
		ScopeType: scopeType,
		ScopeID:   scopeID,
		Key:       dataKey,
		Value:     value,
		UpdatedBy: actorProfileID,
	}
	if err := s.store.PutPluginDatum(ctx, datum); err != nil {
		return core.PluginDatum{}, err
	}
	return datum, nil
}

// ListData returns everything a plugin has stored against one scope.
func (s *PluginService) ListData(ctx context.Context, actorProfileID, pluginID, scopeType, scopeID string) ([]core.PluginDatum, error) {
	if err := s.requireStorageAccess(ctx, actorProfileID, pluginID, scopeType, scopeID, false); err != nil {
		return nil, err
	}
	return s.store.ListPluginData(ctx, pluginID, scopeType, scopeID)
}

// DeleteDatum removes one stored value.
func (s *PluginService) DeleteDatum(ctx context.Context, actorProfileID, pluginID, scopeType, scopeID, dataKey string) error {
	if err := s.requireStorageAccess(ctx, actorProfileID, pluginID, scopeType, scopeID, true); err != nil {
		return err
	}
	return s.store.DeletePluginDatum(ctx, pluginID, scopeType, scopeID, dataKey)
}

// requireStorageAccess enforces two separate questions, both of which must pass.
//
// First, is this plugin allowed to store anything at all for this actor? That means the
// actor has it installed somewhere they control, with storage granted. Second, may the
// actor touch this particular scope? Writing to a loadout requires owning it; reading one
// requires being able to see it.
func (s *PluginService) requireStorageAccess(ctx context.Context, actorProfileID, pluginID, scopeType, scopeID string, write bool) error {
	if actorProfileID == "" {
		return fmt.Errorf("%w: plugin storage requires a profile", core.ErrForbidden)
	}
	if strings.TrimSpace(scopeID) == "" {
		return fmt.Errorf("%w: a scope id is required", core.ErrInvalid)
	}

	if err := s.requireStorageGrant(ctx, actorProfileID, pluginID, scopeType, scopeID); err != nil {
		return err
	}

	switch scopeType {
	case core.ScopeLoadout:
		loadout, err := s.store.GetLoadout(ctx, scopeID)
		if err != nil {
			return fmt.Errorf("%w: loadout %s", core.ErrNotFound, scopeID)
		}
		if write {
			if loadout.OwnerProfileID != actorProfileID {
				return fmt.Errorf("%w: only the owner may store plugin data on this loadout", core.ErrForbidden)
			}
			return nil
		}
		if !loadout.IsVisibleTo(actorProfileID) {
			return fmt.Errorf("%w: loadout %s is private", core.ErrForbidden, scopeID)
		}
		return nil

	case core.ScopeProfile:
		if scopeID != actorProfileID {
			return fmt.Errorf("%w: you may only touch plugin data on your own profile", core.ErrForbidden)
		}
		return nil

	case core.ScopeCommunity:
		if write {
			return s.community.RequireAdmin(ctx, scopeID, actorProfileID)
		}
		return nil

	case core.ScopeItem:
		// Items are global, so a writable item scope would let any installer edit data
		// everyone else sees. Until there is moderation for it, item-scoped plugin data
		// is read-only.
		if write {
			return fmt.Errorf("%w: item-scoped plugin data is read-only for now; store it on a loadout or a profile instead",
				core.ErrForbidden)
		}
		return nil
	}
	return fmt.Errorf("%w: unknown storage scope %q", core.ErrInvalid, scopeType)
}

// requireStorageGrant checks the actor has this plugin installed with storage granted,
// either on their own profile or in a community they administer.
func (s *PluginService) requireStorageGrant(ctx context.Context, actorProfileID, pluginID, scopeType, scopeID string) error {
	mine, err := s.store.FindPluginInstall(ctx, core.ScopeProfile, actorProfileID, pluginID)
	if err == nil && mine != nil && mine.Enabled && mine.GrantedCaps.Contains("storage:write") {
		return nil
	}
	// A community install covers its own community scope and anything posted to it.
	communityID := ""
	switch scopeType {
	case core.ScopeCommunity:
		communityID = scopeID
	case core.ScopeLoadout:
		if loadout, err := s.store.GetLoadout(ctx, scopeID); err == nil {
			communityID = loadout.CommunityID
		}
	}
	if communityID != "" {
		theirs, err := s.store.FindPluginInstall(ctx, core.ScopeCommunity, communityID, pluginID)
		if err == nil && theirs != nil && theirs.Enabled && theirs.GrantedCaps.Contains("storage:write") {
			return nil
		}
	}
	return fmt.Errorf("%w: this plugin is not installed with storage access", core.ErrForbidden)
}

// jsonSize reports the encoded size of a value, used to bound plugin storage.
func jsonSize(v interface{}) int {
	b, err := json.Marshal(v)
	if err != nil {
		return 0
	}
	return len(b)
}

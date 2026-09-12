package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/gmccloskey/loadouts/backend/internal/core"
	"github.com/gmccloskey/loadouts/backend/internal/db"
)

// PluginService owns the plugin lifecycle: publishing versions, installing them into a
// scope, and granting the capabilities they ask for.
//
// The whole design assumes plugin authors are untrusted. Three rules follow from that and
// are enforced here rather than in handlers, so no future endpoint can forget them:
//
//  1. Versions are immutable and installs pin one. Publishing v2 cannot change the
//     behaviour an installer already approved.
//  2. Capabilities are default-deny and are recorded on the install. A manifest that
//     starts asking for more forces a re-grant instead of silently widening access.
//  3. Nothing an author writes is ever executed by the host. Widgets are data; embeds run
//     in a sandboxed frame the browser isolates.
type PluginService struct {
	store     db.Store
	community *CommunityService
	loadouts  *LoadoutService
	items     *InventoryService
	// sandboxBase is the origin embed frames are served from. Frames are sandboxed
	// without allow-same-origin, so they already have an opaque origin; serving them
	// from a separate host as well is defence in depth.
	sandboxBase string
}

func NewPluginService(store db.Store, community *CommunityService, loadouts *LoadoutService, items *InventoryService, sandboxBase string) *PluginService {
	return &PluginService{store: store, community: community, loadouts: loadouts, items: items, sandboxBase: sandboxBase}
}

// maxSettingLength bounds a per-install setting value. Settings hold API keys and
// thresholds, not documents.
const maxSettingLength = 4096

// PublishRequest creates a plugin and its first version in one call.
type PublishRequest struct {
	Slug        string              `json:"slug"`
	Name        string              `json:"name"`
	Description string              `json:"description"`
	OwnerType   core.OwnerType      `json:"owner_type"`
	OwnerID     string              `json:"owner_id"`
	IsPublic    bool                `json:"is_public"`
	Manifest    core.PluginManifest `json:"manifest"`
	Changelog   string              `json:"changelog"`
}

// InstallRequest enables a plugin in a scope.
type InstallRequest struct {
	PluginID string `json:"plugin_id"`
	// Version pins the install. Zero means the plugin's latest at install time.
	Version   int    `json:"version"`
	ScopeType string `json:"scope_type"`
	ScopeID   string `json:"scope_id"`
	// GrantedCaps must cover everything the manifest asks for, so a user always sees
	// the full list before anything is enabled.
	GrantedCaps []string      `json:"granted_caps"`
	Settings    core.Metadata `json:"settings"`
}

// InstallView is an install plus everything the UI needs to explain it.
type InstallView struct {
	Install  core.PluginInstall  `json:"install"`
	Plugin   core.Plugin         `json:"plugin"`
	Manifest core.PluginManifest `json:"manifest"`
	// UpgradeAvailable reports that the author has published past the pinned version.
	UpgradeAvailable bool `json:"upgrade_available"`
	// MissingCaps lists what the latest version would additionally need. A non-empty
	// list means upgrading requires the installer to approve more access.
	MissingCaps []string `json:"missing_caps,omitempty"`
}

// --- Publishing ---

// Create registers a plugin and publishes version 1.
func (s *PluginService) Create(ctx context.Context, actorProfileID string, req PublishRequest) (core.PluginDetail, error) {
	if actorProfileID == "" {
		return core.PluginDetail{}, fmt.Errorf("%w: a profile is required to publish a plugin", core.ErrForbidden)
	}
	if strings.TrimSpace(req.Name) == "" {
		return core.PluginDetail{}, fmt.Errorf("%w: name is required", core.ErrInvalid)
	}
	if err := core.ValidateManifest(req.Manifest); err != nil {
		return core.PluginDetail{}, err
	}

	plugin := core.Plugin{
		ID:            core.NewID("plg"),
		Slug:          core.Slugify(defaultString(req.Slug, req.Name)),
		Name:          req.Name,
		Description:   req.Description,
		OwnerType:     req.OwnerType,
		OwnerID:       req.OwnerID,
		LatestVersion: 1,
		IsPublic:      req.IsPublic,
	}

	switch req.OwnerType {
	case core.OwnerCommunity:
		// A community-owned plugin is a statement by that community, so publishing one
		// requires admin rights there.
		if err := s.community.RequireAdmin(ctx, req.OwnerID, actorProfileID); err != nil {
			return core.PluginDetail{}, err
		}
	case core.OwnerPlatform:
		return core.PluginDetail{}, fmt.Errorf("%w: platform plugins are seeded, not user-created", core.ErrForbidden)
	default:
		plugin.OwnerType = core.OwnerProfile
		plugin.OwnerID = actorProfileID
	}

	if plugin.Slug == "" {
		return core.PluginDetail{}, fmt.Errorf("%w: could not derive a slug from %q", core.ErrInvalid, req.Name)
	}
	if _, err := s.store.GetPluginBySlug(ctx, plugin.Slug); err == nil {
		return core.PluginDetail{}, fmt.Errorf("%w: the slug %q is taken", core.ErrConflict, plugin.Slug)
	}

	if err := s.store.CreatePlugin(ctx, plugin); err != nil {
		return core.PluginDetail{}, err
	}

	version := core.PluginVersion{
		PluginID:  plugin.ID,
		Version:   1,
		Manifest:  req.Manifest,
		Changelog: defaultString(req.Changelog, "Initial version."),
		CreatedBy: actorProfileID,
	}
	if err := s.store.CreatePluginVersion(ctx, version); err != nil {
		return core.PluginDetail{}, err
	}
	return core.PluginDetail{Plugin: plugin, Version: version}, nil
}

// PublishVersion appends an immutable version and advances the latest pointer. Existing
// installs keep running their pinned version until somebody upgrades them.
func (s *PluginService) PublishVersion(ctx context.Context, actorProfileID, pluginID string, manifest core.PluginManifest, changelog string) (core.PluginDetail, error) {
	plugin, err := s.store.GetPlugin(ctx, pluginID)
	if err != nil {
		return core.PluginDetail{}, fmt.Errorf("%w: plugin %s", core.ErrNotFound, pluginID)
	}
	if err := s.requireAuthor(ctx, plugin, actorProfileID); err != nil {
		return core.PluginDetail{}, err
	}
	if err := core.ValidateManifest(manifest); err != nil {
		return core.PluginDetail{}, err
	}

	next := core.PluginVersion{
		PluginID:  plugin.ID,
		Version:   plugin.LatestVersion + 1,
		Manifest:  manifest,
		Changelog: defaultString(changelog, fmt.Sprintf("Version %d", plugin.LatestVersion+1)),
		CreatedBy: actorProfileID,
	}
	if err := s.store.CreatePluginVersion(ctx, next); err != nil {
		return core.PluginDetail{}, err
	}

	plugin.LatestVersion = next.Version
	if err := s.store.UpdatePlugin(ctx, plugin); err != nil {
		return core.PluginDetail{}, err
	}
	return core.PluginDetail{Plugin: plugin, Version: next}, nil
}

// SetVisibility flips a plugin between unlisted and listed.
func (s *PluginService) SetVisibility(ctx context.Context, actorProfileID, pluginID string, isPublic bool) (core.Plugin, error) {
	plugin, err := s.store.GetPlugin(ctx, pluginID)
	if err != nil {
		return core.Plugin{}, fmt.Errorf("%w: plugin %s", core.ErrNotFound, pluginID)
	}
	if err := s.requireAuthor(ctx, plugin, actorProfileID); err != nil {
		return core.Plugin{}, err
	}
	plugin.IsPublic = isPublic
	if err := s.store.UpdatePlugin(ctx, plugin); err != nil {
		return core.Plugin{}, err
	}
	return plugin, nil
}

func (s *PluginService) requireAuthor(ctx context.Context, plugin core.Plugin, actorProfileID string) error {
	switch plugin.OwnerType {
	case core.OwnerProfile:
		if actorProfileID == "" || plugin.OwnerID != actorProfileID {
			return fmt.Errorf("%w: only the author may publish new versions of this plugin", core.ErrForbidden)
		}
		return nil
	case core.OwnerCommunity:
		return s.community.RequireAdmin(ctx, plugin.OwnerID, actorProfileID)
	default:
		return fmt.Errorf("%w: platform plugins are immutable", core.ErrForbidden)
	}
}

// --- Reading ---

// Detail returns a plugin with one version. Pass version <= 0 for the latest.
func (s *PluginService) Detail(ctx context.Context, pluginID string, version int) (core.PluginDetail, error) {
	plugin, err := s.store.GetPlugin(ctx, pluginID)
	if err != nil {
		return core.PluginDetail{}, fmt.Errorf("%w: plugin %s", core.ErrNotFound, pluginID)
	}
	if version <= 0 {
		version = plugin.LatestVersion
	}
	pv, err := s.store.GetPluginVersion(ctx, pluginID, version)
	if err != nil {
		return core.PluginDetail{}, fmt.Errorf("%w: plugin %s v%d", core.ErrNotFound, pluginID, version)
	}
	return core.PluginDetail{Plugin: plugin, Version: pv}, nil
}

// List returns the plugin directory.
func (s *PluginService) List(ctx context.Context, q core.PluginQuery) ([]core.PluginDetail, error) {
	plugins, err := s.store.ListPlugins(ctx, q)
	if err != nil {
		return nil, err
	}
	details := make([]core.PluginDetail, 0, len(plugins))
	for _, p := range plugins {
		detail, err := s.Detail(ctx, p.ID, p.LatestVersion)
		if err != nil {
			// A plugin row with no readable version is a broken record, not a reason
			// to fail the whole directory.
			continue
		}
		details = append(details, detail)
	}
	return details, nil
}

// Versions lists a plugin's published history.
func (s *PluginService) Versions(ctx context.Context, pluginID string) ([]core.PluginVersion, error) {
	return s.store.ListPluginVersions(ctx, pluginID)
}

// --- Installing ---

// Install enables a plugin in a profile or community scope.
func (s *PluginService) Install(ctx context.Context, actorProfileID string, req InstallRequest) (InstallView, error) {
	if actorProfileID == "" {
		return InstallView{}, fmt.Errorf("%w: a profile is required to install a plugin", core.ErrForbidden)
	}
	if err := s.requireScopeAdmin(ctx, actorProfileID, req.ScopeType, req.ScopeID); err != nil {
		return InstallView{}, err
	}

	plugin, err := s.store.GetPlugin(ctx, req.PluginID)
	if err != nil {
		return InstallView{}, fmt.Errorf("%w: plugin %s", core.ErrNotFound, req.PluginID)
	}
	// An unlisted plugin is installable by its author, which is how you test one before
	// sharing it.
	if !plugin.IsPublic {
		if err := s.requireAuthor(ctx, plugin, actorProfileID); err != nil {
			return InstallView{}, fmt.Errorf("%w: plugin %s is not published", core.ErrForbidden, plugin.Slug)
		}
	}

	version := req.Version
	if version <= 0 {
		version = plugin.LatestVersion
	}
	pv, err := s.store.GetPluginVersion(ctx, plugin.ID, version)
	if err != nil {
		return InstallView{}, fmt.Errorf("%w: plugin %s v%d", core.ErrNotFound, plugin.Slug, version)
	}

	granted := core.StringList(req.GrantedCaps)
	if missing := granted.MissingFrom(pv.Manifest.Capabilities.List()); len(missing) > 0 {
		return InstallView{}, fmt.Errorf("%w: this plugin also needs %s, which was not granted",
			core.ErrInvalid, strings.Join(missing, ", "))
	}
	settings, err := normalizeSettings(pv.Manifest.Settings, req.Settings)
	if err != nil {
		return InstallView{}, err
	}

	// Re-installing upgrades in place: keep the existing row's ID so anything keyed to
	// the install survives the upgrade.
	id := core.NewID("pin")
	if existing, err := s.store.FindPluginInstall(ctx, req.ScopeType, req.ScopeID, plugin.ID); err == nil && existing != nil {
		id = existing.ID
	}

	install := core.PluginInstall{
		ID:          id,
		PluginID:    plugin.ID,
		Version:     version,
		ScopeType:   req.ScopeType,
		ScopeID:     req.ScopeID,
		GrantedCaps: granted,
		Settings:    settings,
		Enabled:     true,
		InstalledBy: actorProfileID,
	}
	if err := s.store.UpsertPluginInstall(ctx, install); err != nil {
		return InstallView{}, err
	}
	return s.viewInstall(ctx, install)
}

// Uninstall removes an install. Plugin data survives, so reinstalling restores it.
func (s *PluginService) Uninstall(ctx context.Context, actorProfileID, installID string) error {
	install, err := s.store.GetPluginInstall(ctx, installID)
	if err != nil {
		return fmt.Errorf("%w: install %s", core.ErrNotFound, installID)
	}
	if err := s.requireScopeAdmin(ctx, actorProfileID, install.ScopeType, install.ScopeID); err != nil {
		return err
	}
	return s.store.DeletePluginInstall(ctx, installID)
}

// SetEnabled toggles an install without discarding its settings.
func (s *PluginService) SetEnabled(ctx context.Context, actorProfileID, installID string, enabled bool) (InstallView, error) {
	install, err := s.store.GetPluginInstall(ctx, installID)
	if err != nil {
		return InstallView{}, fmt.Errorf("%w: install %s", core.ErrNotFound, installID)
	}
	if err := s.requireScopeAdmin(ctx, actorProfileID, install.ScopeType, install.ScopeID); err != nil {
		return InstallView{}, err
	}
	install.Enabled = enabled
	if err := s.store.UpsertPluginInstall(ctx, install); err != nil {
		return InstallView{}, err
	}
	return s.viewInstall(ctx, install)
}

// UpdateSettings replaces an install's configuration.
func (s *PluginService) UpdateSettings(ctx context.Context, actorProfileID, installID string, settings core.Metadata) (InstallView, error) {
	install, err := s.store.GetPluginInstall(ctx, installID)
	if err != nil {
		return InstallView{}, fmt.Errorf("%w: install %s", core.ErrNotFound, installID)
	}
	if err := s.requireScopeAdmin(ctx, actorProfileID, install.ScopeType, install.ScopeID); err != nil {
		return InstallView{}, err
	}
	pv, err := s.store.GetPluginVersion(ctx, install.PluginID, install.Version)
	if err != nil {
		return InstallView{}, fmt.Errorf("%w: plugin version", core.ErrNotFound)
	}
	normalized, err := normalizeSettings(pv.Manifest.Settings, settings)
	if err != nil {
		return InstallView{}, err
	}
	install.Settings = normalized
	if err := s.store.UpsertPluginInstall(ctx, install); err != nil {
		return InstallView{}, err
	}
	return s.viewInstall(ctx, install)
}

// ListInstalls returns everything installed in a scope.
func (s *PluginService) ListInstalls(ctx context.Context, scopeType, scopeID string) ([]InstallView, error) {
	installs, err := s.store.ListPluginInstalls(ctx, scopeType, scopeID)
	if err != nil {
		return nil, err
	}
	views := make([]InstallView, 0, len(installs))
	for _, i := range installs {
		view, err := s.viewInstall(ctx, i)
		if err != nil {
			continue
		}
		views = append(views, view)
	}
	return views, nil
}

func (s *PluginService) viewInstall(ctx context.Context, install core.PluginInstall) (InstallView, error) {
	plugin, err := s.store.GetPlugin(ctx, install.PluginID)
	if err != nil {
		return InstallView{}, fmt.Errorf("%w: plugin %s", core.ErrNotFound, install.PluginID)
	}
	pv, err := s.store.GetPluginVersion(ctx, install.PluginID, install.Version)
	if err != nil {
		return InstallView{}, fmt.Errorf("%w: plugin %s v%d", core.ErrNotFound, install.PluginID, install.Version)
	}

	view := InstallView{
		Install:          install,
		Plugin:           plugin,
		Manifest:         pv.Manifest,
		UpgradeAvailable: plugin.LatestVersion > install.Version,
	}
	// Surface what an upgrade would newly require, so the UI can warn before offering it.
	if view.UpgradeAvailable {
		if latest, err := s.store.GetPluginVersion(ctx, plugin.ID, plugin.LatestVersion); err == nil {
			view.MissingCaps = install.GrantedCaps.MissingFrom(latest.Manifest.Capabilities.List())
		}
	}
	view.Install.Settings = redactSecrets(pv.Manifest.Settings, install.Settings)
	return view, nil
}

// requireScopeAdmin answers "may this profile change what is installed here?".
func (s *PluginService) requireScopeAdmin(ctx context.Context, actorProfileID, scopeType, scopeID string) error {
	switch scopeType {
	case core.ScopeProfile:
		if actorProfileID == "" || scopeID != actorProfileID {
			return fmt.Errorf("%w: you may only manage plugins on your own profile", core.ErrForbidden)
		}
		return nil
	case core.ScopeCommunity:
		return s.community.RequireAdmin(ctx, scopeID, actorProfileID)
	default:
		return fmt.Errorf("%w: plugins install to a %q or a %q, not a %q",
			core.ErrInvalid, core.ScopeProfile, core.ScopeCommunity, scopeType)
	}
}

// normalizeSettings checks submitted settings against the manifest's declarations.
// Undeclared keys are dropped rather than rejected, so an install survives a version that
// removes a setting.
func normalizeSettings(defs []core.SettingDefinition, submitted core.Metadata) (core.Metadata, error) {
	out := core.Metadata{}
	for _, def := range defs {
		raw, present := submitted[def.Key]
		value := ""
		if present && raw != nil {
			value = fmt.Sprint(raw)
		}
		if strings.TrimSpace(value) == "" && def.Default != "" {
			value = def.Default
		}
		label := defaultString(def.Label, def.Key)

		if def.Required && strings.TrimSpace(value) == "" {
			return nil, fmt.Errorf("%w: %q is required", core.ErrInvalid, label)
		}
		if len(value) > maxSettingLength {
			return nil, fmt.Errorf("%w: %q is longer than %d characters", core.ErrInvalid, label, maxSettingLength)
		}

		switch def.Type {
		case core.SettingNumber:
			if value != "" {
				f, err := parseSettingNumber(value)
				if err != nil {
					return nil, fmt.Errorf("%w: %q must be a number", core.ErrInvalid, label)
				}
				out[def.Key] = f
			}
			continue
		case core.SettingBool:
			out[def.Key] = value == "true" || value == "1"
			continue
		case core.SettingSelect:
			if value != "" && !core.StringList(def.Options).Contains(value) {
				return nil, fmt.Errorf("%w: %q must be one of %s",
					core.ErrInvalid, label, strings.Join(def.Options, ", "))
			}
		}
		if value != "" {
			out[def.Key] = value
		}
	}
	return out, nil
}

func parseSettingNumber(v string) (float64, error) {
	var f float64
	if _, err := fmt.Sscanf(strings.TrimSpace(v), "%g", &f); err != nil {
		return 0, err
	}
	return f, nil
}

// redactSecrets blanks secret settings in API responses.
//
// The frame itself still receives the real value, because a Maps API key is only useful
// in the browser. What this prevents is a secret leaking through a listing endpoint to
// someone who is merely browsing a community's installed plugins.
func redactSecrets(defs []core.SettingDefinition, settings core.Metadata) core.Metadata {
	out := core.Metadata{}
	for k, v := range settings {
		out[k] = v
	}
	for _, def := range defs {
		if def.Type != core.SettingSecret {
			continue
		}
		if v, ok := out[def.Key]; ok && fmt.Sprint(v) != "" {
			out[def.Key] = redactedSecret
		}
	}
	return out
}

const redactedSecret = "••••••"

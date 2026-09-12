package core

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// A Plugin extends the Loadouts UI with views: charts, calculators, data tables, maps.
//
// Plugins are authored by untrusted users and communities, which is the single constraint
// that shapes the whole design. There are two execution tiers (see ViewKind): a
// declarative widget tier where no author code ever runs, and a sandboxed embed tier for
// plugins that genuinely need to execute JavaScript.
//
// A plugin owns both halves of a concept: the metadata schema that describes gear a
// certain way (see SchemaDefinition) and the views that render it. That makes a plugin
// the natural unit for "this community's way of looking at gear".
type Plugin struct {
	ID          string    `json:"id" db:"id"`
	Slug        string    `json:"slug" db:"slug"`
	Name        string    `json:"name" db:"name"`
	Description string    `json:"description" db:"description"`
	OwnerType   OwnerType `json:"owner_type" db:"owner_type"`
	OwnerID     string    `json:"owner_id" db:"owner_id"`
	// LatestVersion is the highest published version. Installs pin their own version.
	LatestVersion int       `json:"latest_version" db:"latest_version"`
	IsPublic      bool      `json:"is_public" db:"is_public"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time `json:"updated_at" db:"updated_at"`
}

// PluginVersion is an immutable published manifest. Publishing a new version never
// mutates an old one, so an install pinned to v1 keeps behaving exactly as approved.
type PluginVersion struct {
	PluginID  string         `json:"plugin_id" db:"plugin_id"`
	Version   int            `json:"version" db:"version"`
	Manifest  PluginManifest `json:"manifest" db:"manifest"`
	Changelog string         `json:"changelog" db:"changelog"`
	CreatedBy string         `json:"created_by" db:"created_by"`
	CreatedAt time.Time      `json:"created_at" db:"created_at"`
}

// PluginDetail bundles a plugin with one resolved version.
type PluginDetail struct {
	Plugin  Plugin        `json:"plugin"`
	Version PluginVersion `json:"version"`
}

// ManifestAPIVersion is the host contract version. A manifest declaring a newer version
// than the host understands is rejected at publish time rather than failing at render.
const ManifestAPIVersion = 1

// PluginManifest is everything a plugin declares about itself.
type PluginManifest struct {
	APIVersion int `json:"api_version"`
	// Views are the things this plugin renders, each bound to a surface.
	Views []PluginView `json:"views"`
	// Capabilities are requested at publish time and granted at install time.
	Capabilities Capabilities `json:"capabilities"`
	// Schemas are metadata namespaces this plugin owns and validates.
	Schemas []PluginSchema `json:"schemas,omitempty"`
	// Settings are per-install configuration fields (API keys, toggles, thresholds).
	Settings []SettingDefinition `json:"settings,omitempty"`
}

func (m PluginManifest) Value() (driver.Value, error) { return json.Marshal(m) }

func (m *PluginManifest) Scan(value interface{}) error {
	b, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("plugin manifest: type assertion to []byte failed")
	}
	return json.Unmarshal(b, m)
}

// PluginSchema associates a metadata namespace with a JSON Schema the plugin enforces.
type PluginSchema struct {
	// Namespace is the metadata key this plugin owns, e.g. "ul" for ul.ul_score.
	Namespace string `json:"namespace"`
	// Definition is a JSON Schema document, validated through core.ValidateMetadata.
	Definition json.RawMessage `json:"definition"`
}

// Surface is a named extension point in the host UI.
type Surface string

const (
	SurfaceLoadoutPanel   Surface = "loadout.panel"
	SurfaceLoadoutSidebar Surface = "loadout.sidebar"
	SurfaceItemTab        Surface = "item.tab"
	SurfaceCommunityTab   Surface = "community.tab"
)

// Surfaces lists every extension point the host can render into.
func Surfaces() []Surface {
	return []Surface{SurfaceLoadoutPanel, SurfaceLoadoutSidebar, SurfaceItemTab, SurfaceCommunityTab}
}

// IsValid reports whether the surface is one the host knows how to render.
func (s Surface) IsValid() bool {
	for _, known := range Surfaces() {
		if s == known {
			return true
		}
	}
	return false
}

// ScopeType is what a surface is looking at, and what plugin storage can be keyed to.
func (s Surface) ScopeType() string {
	switch s {
	case SurfaceLoadoutPanel, SurfaceLoadoutSidebar:
		return ScopeLoadout
	case SurfaceItemTab:
		return ScopeItem
	case SurfaceCommunityTab:
		return ScopeCommunity
	}
	return ""
}

// Storage scope types.
const (
	ScopeLoadout   = "loadout"
	ScopeItem      = "item"
	ScopeCommunity = "community"
	ScopeProfile   = "profile"
)

// SupportsSource reports whether a widget on this surface may read a given data source.
//
// A loadout panel cannot ask for the community catalog and an item tab cannot ask for
// loadout entries, because the host simply has nothing to put in those rows. Checking it
// here means the mismatch is a publish-time error instead of an empty widget nobody can
// explain.
func (s Surface) SupportsSource(source WidgetSource) bool {
	switch s.ScopeType() {
	case ScopeLoadout:
		return source == SourceLoadoutEntries || source == SourceLoadoutStats
	case ScopeItem:
		return source == SourceItem
	case ScopeCommunity:
		return source == SourceCommunityItems
	}
	return false
}

// ViewKind selects the execution tier.
type ViewKind string

const (
	// ViewWidget is declarative: a JSON spec evaluated server-side and drawn by host
	// components. No author code executes anywhere, so it needs no trust at all.
	ViewWidget ViewKind = "widget"
	// ViewEmbed is author HTML/JS in a sandboxed iframe. Powerful enough for Google Maps
	// or a custom visualization, isolated by the sandbox and gated by capabilities.
	ViewEmbed ViewKind = "embed"
)

// PluginView is one renderable thing a plugin contributes.
type PluginView struct {
	ID      string   `json:"id"`
	Title   string   `json:"title"`
	Surface Surface  `json:"surface"`
	Kind    ViewKind `json:"kind"`
	// Widget is the declarative spec, required when Kind is ViewWidget.
	Widget *WidgetSpec `json:"widget,omitempty"`
	// HTML is the embed document, required when Kind is ViewEmbed. It is stored inline
	// rather than fetched from an author-hosted URL so that authoring a plugin does not
	// require hosting anything.
	HTML string `json:"html,omitempty"`
	// Height is the rendered height in pixels; embeds may request a resize at runtime.
	Height int `json:"height,omitempty"`
}

// Capabilities is what a plugin asks for, and what an installer grants.
//
// Everything is default-deny: a plugin that declares nothing receives a context with no
// loadout, no items, and no storage access.
type Capabilities struct {
	ReadLoadout   bool `json:"read_loadout"`
	ReadItems     bool `json:"read_items"`
	ReadCommunity bool `json:"read_community"`
	// Storage allows plugin-namespaced persistence, isolated per plugin and scope.
	Storage bool `json:"storage"`
	// Network is the allowlist of outbound hosts an embed may contact. It becomes the
	// sandbox frame's Content-Security-Policy, so it is enforced by the browser rather
	// than by our good intentions.
	Network []string `json:"network,omitempty"`
}

// List renders capabilities as stable string identifiers, for grant records and for
// showing the user what they are approving.
func (c Capabilities) List() []string {
	out := []string{}
	if c.ReadLoadout {
		out = append(out, "loadout:read")
	}
	if c.ReadItems {
		out = append(out, "items:read")
	}
	if c.ReadCommunity {
		out = append(out, "community:read")
	}
	if c.Storage {
		out = append(out, "storage:write")
	}
	for _, host := range c.Network {
		out = append(out, "network:"+host)
	}
	return out
}

// SettingType constrains what a per-install setting can hold.
type SettingType string

const (
	SettingText   SettingType = "text"
	SettingSecret SettingType = "secret" // e.g. a Maps API key; write-only to non-installers
	SettingNumber SettingType = "number"
	SettingBool   SettingType = "bool"
	SettingSelect SettingType = "select"
)

// SettingDefinition describes one per-install configuration field.
type SettingDefinition struct {
	Key      string      `json:"key"`
	Label    string      `json:"label"`
	Type     SettingType `json:"type"`
	Required bool        `json:"required"`
	Default  string      `json:"default,omitempty"`
	Help     string      `json:"help,omitempty"`
	Options  []string    `json:"options,omitempty"`
}

// PluginInstall is a plugin enabled in a scope.
//
// The pinned Version is deliberate: a plugin author publishing v2 must not silently
// change behaviour the installer already approved, exactly as loadouts pin template
// versions.
type PluginInstall struct {
	ID       string `json:"id" db:"id"`
	PluginID string `json:"plugin_id" db:"plugin_id"`
	Version  int    `json:"version" db:"version"`
	// ScopeType is ScopeProfile or ScopeCommunity.
	ScopeType string `json:"scope_type" db:"scope_type"`
	ScopeID   string `json:"scope_id" db:"scope_id"`
	// GrantedCaps records what was approved, so a manifest asking for more after an
	// upgrade forces a re-grant instead of silently widening access.
	GrantedCaps StringList `json:"granted_caps" db:"granted_caps"`
	Settings    Metadata   `json:"settings" db:"settings"`
	Enabled     bool       `json:"enabled" db:"enabled"`
	InstalledBy string     `json:"installed_by" db:"installed_by"`
	CreatedAt   time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at" db:"updated_at"`
}

// StringList is a JSONB-backed []string.
type StringList []string

func (s StringList) Value() (driver.Value, error) {
	if s == nil {
		return json.Marshal([]string{})
	}
	return json.Marshal(s)
}

func (s *StringList) Scan(value interface{}) error {
	b, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("string list: type assertion to []byte failed")
	}
	return json.Unmarshal(b, s)
}

// Contains reports membership.
func (s StringList) Contains(v string) bool {
	for _, item := range s {
		if item == v {
			return true
		}
	}
	return false
}

// MissingFrom returns the entries of want that are absent from s.
func (s StringList) MissingFrom(want []string) []string {
	var missing []string
	for _, w := range want {
		if !s.Contains(w) {
			missing = append(missing, w)
		}
	}
	return missing
}

// PluginDatum is plugin-namespaced storage. The (plugin, scope, key) composite is what
// keeps one plugin from reading another's data, and one loadout's data from leaking into
// another's.
type PluginDatum struct {
	PluginID  string    `json:"plugin_id" db:"plugin_id"`
	ScopeType string    `json:"scope_type" db:"scope_type"`
	ScopeID   string    `json:"scope_id" db:"scope_id"`
	Key       string    `json:"key" db:"key"`
	Value     Metadata  `json:"value" db:"value"`
	UpdatedBy string    `json:"updated_by" db:"updated_by"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// PluginQuery filters a plugin listing.
type PluginQuery struct {
	Search      string
	OwnerType   OwnerType
	OwnerID     string
	Surface     Surface
	PublicOnly  bool
	IncludeMine string // profile ID whose private plugins should also be returned
}

// RenderedView is what the host hands the frontend for one installed view.
//
// Widget views arrive fully evaluated, so the frontend never sees an expression. Embed
// views arrive as a sandbox URL plus the context the frame will be given.
type RenderedView struct {
	InstallID string   `json:"install_id"`
	PluginID  string   `json:"plugin_id"`
	Version   int      `json:"version"`
	ViewID    string   `json:"view_id"`
	Title     string   `json:"title"`
	Surface   Surface  `json:"surface"`
	Kind      ViewKind `json:"kind"`
	Height    int      `json:"height,omitempty"`

	// Widget tier: the evaluated render model.
	Widget *WidgetRender `json:"widget,omitempty"`

	// Embed tier: where to point the iframe, and what to post into it.
	EmbedURL     string                 `json:"embed_url,omitempty"`
	EmbedContext map[string]interface{} `json:"embed_context,omitempty"`

	// Error explains a view that could not be rendered, so one broken plugin degrades to
	// a message in its own card instead of failing the whole page.
	Error string `json:"error,omitempty"`
}

// ValidateManifest checks a manifest at publish time. Catching problems here rather than
// at render means a broken plugin cannot be published in the first place.
func ValidateManifest(m PluginManifest) error {
	if m.APIVersion == 0 {
		return fmt.Errorf("%w: manifest api_version is required", ErrInvalid)
	}
	if m.APIVersion > ManifestAPIVersion {
		return fmt.Errorf("%w: manifest api_version %d is newer than this host supports (%d)",
			ErrInvalid, m.APIVersion, ManifestAPIVersion)
	}
	if len(m.Views) == 0 {
		return fmt.Errorf("%w: a plugin must declare at least one view", ErrInvalid)
	}

	seen := map[string]bool{}
	for i, v := range m.Views {
		if strings.TrimSpace(v.ID) == "" {
			return fmt.Errorf("%w: view %d has no id", ErrInvalid, i)
		}
		if seen[v.ID] {
			return fmt.Errorf("%w: duplicate view id %q", ErrInvalid, v.ID)
		}
		seen[v.ID] = true

		if !v.Surface.IsValid() {
			return fmt.Errorf("%w: view %q targets unknown surface %q", ErrInvalid, v.ID, v.Surface)
		}

		switch v.Kind {
		case ViewWidget:
			if v.Widget == nil {
				return fmt.Errorf("%w: widget view %q has no widget spec", ErrInvalid, v.ID)
			}
			if err := ValidateWidgetSpec(*v.Widget); err != nil {
				return fmt.Errorf("%w: view %q: %s", ErrInvalid, v.ID, err)
			}
			if !v.Surface.SupportsSource(v.Widget.Source) {
				return fmt.Errorf("%w: view %q reads %q, which the %q surface cannot provide",
					ErrInvalid, v.ID, v.Widget.Source, v.Surface)
			}
		case ViewEmbed:
			if strings.TrimSpace(v.HTML) == "" {
				return fmt.Errorf("%w: embed view %q has no html", ErrInvalid, v.ID)
			}
		default:
			return fmt.Errorf("%w: view %q has unknown kind %q (want %q or %q)",
				ErrInvalid, v.ID, v.Kind, ViewWidget, ViewEmbed)
		}
	}

	for _, s := range m.Schemas {
		if strings.TrimSpace(s.Namespace) == "" {
			return fmt.Errorf("%w: a declared schema has no namespace", ErrInvalid)
		}
		if s.Namespace == CoreNamespace {
			return fmt.Errorf("%w: the %q namespace is reserved by the platform", ErrInvalid, CoreNamespace)
		}
		// A plugin that declares an unusable schema would fail every metadata write it
		// gates, so the schema document itself is compiled here.
		if err := ValidateSchemaDocument(s.Definition); err != nil {
			return fmt.Errorf("%w: schema %q: %s", ErrInvalid, s.Namespace, err)
		}
	}

	for _, s := range m.Settings {
		if strings.TrimSpace(s.Key) == "" {
			return fmt.Errorf("%w: a declared setting has no key", ErrInvalid)
		}
	}

	for _, host := range m.Capabilities.Network {
		if strings.ContainsAny(host, " \t\n;'\"") {
			return fmt.Errorf("%w: network host %q contains illegal characters", ErrInvalid, host)
		}
	}
	return nil
}

// ViewByID finds a view in a manifest.
func (m PluginManifest) ViewByID(id string) (PluginView, bool) {
	for _, v := range m.Views {
		if v.ID == id {
			return v, true
		}
	}
	return PluginView{}, false
}

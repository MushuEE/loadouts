package db

import (
	"context"

	"github.com/gmccloskey/loadouts/backend/internal/core"
)

// Store defines the interface for database operations.
type Store interface {
	// Schemas
	CreateSchema(ctx context.Context, schema core.SchemaDefinition) error
	GetSchema(ctx context.Context, id, version string) (core.SchemaDefinition, error)
	ListSchemas(ctx context.Context) ([]core.SchemaDefinition, error)

	// Items
	CreateItem(ctx context.Context, item core.Item) error
	GetItem(ctx context.Context, id string) (core.Item, error)
	ListItems(ctx context.Context, query string) ([]core.Item, error) // Simplified search

	// Item Sources & Suppliers
	GetItemSources(ctx context.Context, itemID string) ([]core.ItemSource, error)
	UpsertItemSource(ctx context.Context, source core.ItemSource) error
	GetSupplier(ctx context.Context, id string) (core.Supplier, error)
	UpsertSupplier(ctx context.Context, supplier core.Supplier) error
	ListSuppliers(ctx context.Context) ([]core.Supplier, error)
	GetItemBySource(ctx context.Context, supplierID, productID string) (string, error) // Returns itemID

	// User Metadata (legacy compat shape; backed by profile item layers)
	UpdateUserMetadata(ctx context.Context, meta core.UserMetadata) error
	GetUserMetadata(ctx context.Context, userID, itemID string) (*core.UserMetadata, error)

	// Identity
	CreateUser(ctx context.Context, user core.User) error
	GetUser(ctx context.Context, id string) (core.User, error)
	ListUsers(ctx context.Context) ([]core.User, error)
	CreateProfile(ctx context.Context, profile core.Profile) error
	GetProfile(ctx context.Context, id string) (core.Profile, error)
	GetProfileByHandle(ctx context.Context, handle string) (core.Profile, error)
	ListProfiles(ctx context.Context, userID string) ([]core.Profile, error) // userID "" lists all

	// Communities
	CreateCommunity(ctx context.Context, community core.Community) error
	UpdateCommunity(ctx context.Context, community core.Community) error
	GetCommunity(ctx context.Context, id string) (core.Community, error)
	GetCommunityBySlug(ctx context.Context, slug string) (core.Community, error)
	ListCommunities(ctx context.Context, query string) ([]core.Community, error)
	UpsertMembership(ctx context.Context, membership core.CommunityMembership) error
	DeleteMembership(ctx context.Context, communityID, profileID string) error
	GetMembership(ctx context.Context, communityID, profileID string) (*core.CommunityMembership, error)
	ListMemberships(ctx context.Context, communityID, profileID string) ([]core.CommunityMembership, error)

	// Metadata layers
	UpsertCommunityItemLayer(ctx context.Context, layer core.CommunityItemLayer) error
	GetCommunityItemLayer(ctx context.Context, communityID, itemID string) (*core.CommunityItemLayer, error)
	UpsertProfileItemLayer(ctx context.Context, layer core.ProfileItemLayer) error
	GetProfileItemLayer(ctx context.Context, profileID, itemID string) (*core.ProfileItemLayer, error)

	// Templates
	CreateTemplate(ctx context.Context, tmpl core.Template) error
	UpdateTemplate(ctx context.Context, tmpl core.Template) error
	GetTemplate(ctx context.Context, id string) (core.Template, error)
	ListTemplates(ctx context.Context, query core.TemplateQuery) ([]core.Template, error)
	CreateTemplateVersion(ctx context.Context, version core.TemplateVersion) error
	GetTemplateVersion(ctx context.Context, templateID string, version int) (core.TemplateVersion, error)
	ListTemplateVersions(ctx context.Context, templateID string) ([]core.TemplateVersion, error)

	// Loadouts
	CreateLoadout(ctx context.Context, loadout core.Loadout) error
	UpdateLoadout(ctx context.Context, loadout core.Loadout) error
	DeleteLoadout(ctx context.Context, id string) error
	GetLoadout(ctx context.Context, id string) (core.Loadout, error)
	ListLoadouts(ctx context.Context, query core.DiscoverQuery) ([]core.Loadout, error)
	ReplaceLoadoutEntries(ctx context.Context, loadoutID string, entries []core.LoadoutEntry) error
	ListLoadoutEntries(ctx context.Context, loadoutID string) ([]core.LoadoutEntry, error)
	// ListLoadoutEntriesFor batches a whole level of a nested loadout tree, so that
	// resolving depth-5 nesting costs five queries rather than one per node.
	ListLoadoutEntriesFor(ctx context.Context, loadoutIDs []string) (map[string][]core.LoadoutEntry, error)
	GetLoadoutsByIDs(ctx context.Context, ids []string) (map[string]core.Loadout, error)
	// LoadoutsReferencing is the reverse edge of the sub-loadout graph, needed because
	// the depth check has to walk upwards from the parent as well as down from the child.
	LoadoutsReferencing(ctx context.Context, childLoadoutID string) ([]string, error)

	// Favorites
	//
	// A favorite is keyed by (scope, loadout) rather than by its own ID everywhere,
	// because "has this community endorsed this loadout?" is the question every caller
	// actually asks. The ID exists for the row's own sake.
	UpsertFavorite(ctx context.Context, favorite core.Favorite) error
	DeleteFavorite(ctx context.Context, scopeType core.FavoriteScope, scopeID, loadoutID string) error
	GetFavorite(ctx context.Context, scopeType core.FavoriteScope, scopeID, loadoutID string) (*core.Favorite, error)
	ListFavoritesForScope(ctx context.Context, scopeType core.FavoriteScope, scopeID string) ([]core.Favorite, error)
	ListFavoritesForLoadout(ctx context.Context, loadoutID string) ([]core.Favorite, error)

	// Plugins
	CreatePlugin(ctx context.Context, plugin core.Plugin) error
	UpdatePlugin(ctx context.Context, plugin core.Plugin) error
	GetPlugin(ctx context.Context, id string) (core.Plugin, error)
	GetPluginBySlug(ctx context.Context, slug string) (core.Plugin, error)
	ListPlugins(ctx context.Context, query core.PluginQuery) ([]core.Plugin, error)
	CreatePluginVersion(ctx context.Context, version core.PluginVersion) error
	GetPluginVersion(ctx context.Context, pluginID string, version int) (core.PluginVersion, error)
	ListPluginVersions(ctx context.Context, pluginID string) ([]core.PluginVersion, error)

	// Plugin installs (scope type is core.ScopeProfile or core.ScopeCommunity)
	UpsertPluginInstall(ctx context.Context, install core.PluginInstall) error
	DeletePluginInstall(ctx context.Context, id string) error
	GetPluginInstall(ctx context.Context, id string) (core.PluginInstall, error)
	FindPluginInstall(ctx context.Context, scopeType, scopeID, pluginID string) (*core.PluginInstall, error)
	ListPluginInstalls(ctx context.Context, scopeType, scopeID string) ([]core.PluginInstall, error)

	// Plugin storage, namespaced by (plugin, scope) so one plugin cannot read another's
	// data and one loadout's data cannot leak into another's.
	PutPluginDatum(ctx context.Context, datum core.PluginDatum) error
	GetPluginDatum(ctx context.Context, pluginID, scopeType, scopeID, key string) (*core.PluginDatum, error)
	ListPluginData(ctx context.Context, pluginID, scopeType, scopeID string) ([]core.PluginDatum, error)
	DeletePluginDatum(ctx context.Context, pluginID, scopeType, scopeID, key string) error
}

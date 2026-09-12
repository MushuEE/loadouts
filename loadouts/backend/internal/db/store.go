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
	GetSupplier(ctx context.Context, id string) (core.Supplier, error)
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
}

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

	// User Metadata
	UpdateUserMetadata(ctx context.Context, meta core.UserMetadata) error
	GetUserMetadata(ctx context.Context, userID, itemID string) (*core.UserMetadata, error)
}

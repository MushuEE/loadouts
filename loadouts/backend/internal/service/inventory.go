package service

import (
	"context"
	"fmt"

	"github.com/gmccloskey/loadouts/backend/internal/core"
	"github.com/gmccloskey/loadouts/backend/internal/db"
)

type InventoryService struct {
	store db.Store
	bf    *ItemBloomFilter
}

func NewInventoryService(store db.Store) *InventoryService {
	// Initialize with 100k capacity and 1% error rate for the prototype
	bf := NewItemBloomFilter(100000, 0.01)
	return &InventoryService{
		store: store,
		bf:    bf,
	}
}

func (s *InventoryService) Init(ctx context.Context) error {
	return s.bf.PopulateFromDB(ctx, s.store)
}

func (s *InventoryService) RegisterSchema(ctx context.Context, schema core.SchemaDefinition) error {
	// TODO: Add basic JSON Schema validation for the definition itself if needed
	return s.store.CreateSchema(ctx, schema)
}

func (s *InventoryService) GetSchema(ctx context.Context, id, version string) (core.SchemaDefinition, error) {
	return s.store.GetSchema(ctx, id, version)
}

func (s *InventoryService) ListSchemas(ctx context.Context) ([]core.SchemaDefinition, error) {
	return s.store.ListSchemas(ctx)
}

func (s *InventoryService) CreateItem(ctx context.Context, item core.Item) error {
	// 1. Fast Bloom Filter check
	if s.bf.Test(item.ID) {
		// Bloom filter says "maybe exists".
		// We should still check the DB to be 100% sure (avoid false positive rejection).
		_, err := s.store.GetItem(ctx, item.ID)
		if err == nil {
			return fmt.Errorf("item with ID %s already exists", item.ID)
		}
	}

	// 2. Validate each namespace in base_metadata if they match a schema
	for schemaID, metadata := range item.BaseMetadata {
		// Assuming version 'v1' for now, or we might need to store version in the metadata itself
		schema, err := s.store.GetSchema(ctx, schemaID, "v1")
		if err == nil {
			if err := core.ValidateMetadata(schema, metadata); err != nil {
				return fmt.Errorf("metadata validation failed for %s: %w", schemaID, err)
			}
		}
	}

	if err := s.store.CreateItem(ctx, item); err != nil {
		return err
	}

	// 3. Update Bloom Filter on success
	s.bf.Add(item.ID)
	return nil
}

func (s *InventoryService) GetMergedItem(ctx context.Context, itemID, userID string) (core.MergedItem, error) {
	item, err := s.store.GetItem(ctx, itemID)
	if err != nil {
		return core.MergedItem{}, err
	}

	userMeta, err := s.store.GetUserMetadata(ctx, userID, itemID)
	if err != nil {
		// It's fine if user metadata doesn't exist, just merge with nil
		userMeta = nil
	}

	return core.Merge(item, userMeta), nil
}

func (s *InventoryService) UpdateMetadata(ctx context.Context, userID, itemID string, customImageURL string, overrides core.Metadata, openData core.Metadata) error {
	// Validate overrides against schemas
	for schemaID, metadata := range overrides {
		schema, err := s.store.GetSchema(ctx, schemaID, "v1")
		if err == nil {
			if err := core.ValidateMetadata(schema, metadata); err != nil {
				return fmt.Errorf("override validation failed for %s: %w", schemaID, err)
			}
		}
	}

	meta := core.UserMetadata{
		UserID:         userID,
		ItemID:         itemID,
		CustomImageURL: customImageURL,
		Overrides:      overrides,
		OpenData:       openData,
	}

	return s.store.UpdateUserMetadata(ctx, meta)
}

func (s *InventoryService) SearchItems(ctx context.Context, query string) ([]core.Item, error) {
	return s.store.ListItems(ctx, query)
}

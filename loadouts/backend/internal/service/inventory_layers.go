package service

import (
	"context"
	"fmt"

	"github.com/gmccloskey/loadouts/backend/internal/core"
)

// This file layers the Day 0 metadata model onto the existing InventoryService:
//
//	global -> community -> user public -> user private (owner only)
//
// Reads go through ResolveItem so every surface (item detail, garage, loadout entries)
// applies the same rules, including private-layer redaction.

// ResolveItem flattens an item through every layer applicable to the given context.
func (s *InventoryService) ResolveItem(ctx context.Context, itemID string, lctx core.LayerContext) (core.ResolvedItem, error) {
	item, err := s.store.GetItem(ctx, itemID)
	if err != nil {
		return core.ResolvedItem{}, fmt.Errorf("%w: item %s", core.ErrNotFound, itemID)
	}

	in := core.LayerInput{}
	if lctx.CommunityID != "" {
		if layer, err := s.store.GetCommunityItemLayer(ctx, lctx.CommunityID, itemID); err == nil {
			in.Community = layer
		}
	}
	if owner := lctx.Owner(); owner != "" {
		if layer, err := s.store.GetProfileItemLayer(ctx, owner, itemID); err == nil {
			in.Profile = layer
		}
	}

	resolved := core.ResolveLayers(item, in, lctx)
	resolved.Sources = s.resolveSources(ctx, itemID)
	return resolved, nil
}

// ResolveItems batch-resolves items, skipping any that have gone missing.
func (s *InventoryService) ResolveItems(ctx context.Context, itemIDs []string, lctx core.LayerContext) map[string]core.ResolvedItem {
	out := make(map[string]core.ResolvedItem, len(itemIDs))
	for _, id := range itemIDs {
		if _, done := out[id]; done {
			continue
		}
		if resolved, err := s.ResolveItem(ctx, id, lctx); err == nil {
			out[id] = resolved
		}
	}
	return out
}

// resolveSources templates affiliate URLs for every known supplier of an item.
func (s *InventoryService) resolveSources(ctx context.Context, itemID string) []core.ResolvedSource {
	sources, err := s.store.GetItemSources(ctx, itemID)
	if err != nil {
		return nil
	}
	var resolved []core.ResolvedSource
	for _, src := range sources {
		supplier, err := s.store.GetSupplier(ctx, src.SupplierID)
		if err != nil {
			continue
		}
		url, err := core.ResolveSourceURL(supplier, src)
		if err != nil {
			continue
		}
		resolved = append(resolved, core.ResolvedSource{
			SupplierName: supplier.Name,
			Price:        src.Price,
			URL:          url,
		})
	}
	return resolved
}

// SetProfileLayer writes a profile's public overrides and private notes for an item.
// Public namespaces are validated against the schema registry; the private layer is
// deliberately unvalidated (the "OpenSchema wild west").
func (s *InventoryService) SetProfileLayer(ctx context.Context, layer core.ProfileItemLayer) (core.ProfileItemLayer, error) {
	if layer.ProfileID == "" {
		return core.ProfileItemLayer{}, fmt.Errorf("%w: a profile is required", core.ErrForbidden)
	}
	if _, err := s.store.GetItem(ctx, layer.ItemID); err != nil {
		return core.ProfileItemLayer{}, fmt.Errorf("%w: item %s", core.ErrNotFound, layer.ItemID)
	}

	for schemaID, metadata := range layer.PublicMetadata {
		schema, err := s.store.GetSchema(ctx, schemaID, "v1")
		if err != nil {
			continue // Unregistered namespaces are allowed; only known schemas are enforced.
		}
		if err := core.ValidateMetadata(schema, metadata); err != nil {
			return core.ProfileItemLayer{}, fmt.Errorf("%w: %s layer failed validation: %v", core.ErrInvalid, schemaID, err)
		}
	}

	if layer.PublicMetadata == nil {
		layer.PublicMetadata = core.Metadata{}
	}
	if layer.PrivateMetadata == nil {
		layer.PrivateMetadata = core.Metadata{}
	}

	if err := s.store.UpsertProfileItemLayer(ctx, layer); err != nil {
		return core.ProfileItemLayer{}, err
	}
	return layer, nil
}

// GetProfileLayer returns a profile's raw (unmerged) layer for an item, redacting the
// private half when the viewer is not the owner.
func (s *InventoryService) GetProfileLayer(ctx context.Context, profileID, itemID, viewerProfileID string) (core.ProfileItemLayer, error) {
	layer, err := s.store.GetProfileItemLayer(ctx, profileID, itemID)
	if err != nil {
		return core.ProfileItemLayer{ProfileID: profileID, ItemID: itemID,
			PublicMetadata: core.Metadata{}, PrivateMetadata: core.Metadata{}}, nil
	}
	out := *layer
	if viewerProfileID != profileID {
		out.PrivateMetadata = core.Metadata{}
	}
	return out, nil
}

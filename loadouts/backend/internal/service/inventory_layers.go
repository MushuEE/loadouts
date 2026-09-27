package service

import (
	"context"
	"fmt"
	"sort"
	"strings"

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
	resolved.Tags = []string{}
	if owner := lctx.Owner(); owner != "" {
		if tags, err := s.store.GetProfileItemTags(ctx, owner, itemID); err == nil && tags != nil {
			resolved.Tags = tags
		}
	}
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
	if err := core.ValidateImageURL(layer.CustomImageURL); err != nil {
		return core.ProfileItemLayer{}, err
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

// SetItemTags replaces a profile's tags for an item.
func (s *InventoryService) SetItemTags(ctx context.Context, profileID, itemID string, raw []string) ([]string, error) {
	if profileID == "" {
		return nil, fmt.Errorf("%w: a profile is required", core.ErrForbidden)
	}
	tags, err := core.NormalizeTags(raw)
	if err != nil {
		return nil, err
	}
	if _, err := s.store.GetItem(ctx, itemID); err != nil {
		return nil, fmt.Errorf("%w: item %s", core.ErrNotFound, itemID)
	}
	if err := s.store.SetProfileItemTags(ctx, profileID, itemID, tags); err != nil {
		return nil, err
	}
	return tags, nil
}

// ItemTags returns the profile's own tags for an item and how everyone tags it.
func (s *InventoryService) ItemTags(ctx context.Context, profileID, itemID string) (core.ItemTags, error) {
	out := core.ItemTags{Mine: []string{}, Global: []core.TagCount{}}
	if profileID != "" {
		mine, err := s.store.GetProfileItemTags(ctx, profileID, itemID)
		if err != nil {
			return out, err
		}
		if mine != nil {
			out.Mine = mine
		}
	}
	global, err := s.store.CountItemTags(ctx, itemID)
	if err != nil {
		return out, err
	}
	if global != nil {
		out.Global = global
	}
	return out, nil
}

// PopularTags lists tags across every profile, most widely used first, for suggestions.
func (s *InventoryService) PopularTags(ctx context.Context, prefix string, limit int) ([]core.TagCount, error) {
	prefix = strings.TrimLeft(strings.ToLower(strings.TrimSpace(prefix)), "#")
	out, err := s.store.CountTags(ctx, prefix, limit)
	if out == nil {
		out = []core.TagCount{}
	}
	return out, err
}

// SearchGear finds items within a scope: the searcher's own gear, or everyone's.
//
// The scope also decides what a tag means. Under "mine", #springseattle26 matches only if
// you tagged the item with it; under "everyone", if anybody did. Tags are ANDed.
func (s *InventoryService) SearchGear(ctx context.Context, q core.GearQuery) (core.GearSearch, error) {
	// "#springseattle26 hat" in the search box means the tag plus the text.
	var words []string
	raw := append([]string(nil), q.Tags...)
	for _, w := range strings.Fields(q.Text) {
		if strings.HasPrefix(w, "#") {
			raw = append(raw, w)
		} else {
			words = append(words, w)
		}
	}
	q.Text = strings.Join(words, " ")
	tags, err := core.NormalizeTags(raw)
	if err != nil {
		return core.GearSearch{}, err
	}
	category := strings.ToLower(strings.TrimSpace(q.Category))
	out := core.GearSearch{Scope: q.Scope, Results: []core.GearResult{}, TagFacets: []core.TagCount{}, CategoryFacets: []core.CategoryCount{}}

	items, err := s.store.ListItems(ctx, strings.TrimSpace(q.Text))
	if err != nil {
		return out, err
	}
	mine := map[string]bool{}
	if q.ProfileID != "" {
		ids, err := s.store.ProfileItemIDs(ctx, q.ProfileID)
		if err != nil {
			return out, err
		}
		for _, id := range ids {
			mine[id] = true
		}
	}
	if q.Scope == core.ScopeMine {
		inScope := items[:0]
		for _, it := range items {
			if mine[it.ID] {
				inScope = append(inScope, it)
			}
		}
		items = inScope
	}

	ids := make([]string, len(items))
	for i, it := range items {
		ids[i] = it.ID
	}
	global, err := s.store.CountTagsForItems(ctx, ids)
	if err != nil {
		return out, err
	}
	myTags := map[string][]string{}
	if q.ProfileID != "" {
		if myTags, err = s.store.ProfileTagsForItems(ctx, q.ProfileID, ids); err != nil {
			return out, err
		}
	}
	// scopeTags is what a tag means under the scope.
	scopeTags := func(id string) []string {
		if q.Scope == core.ScopeMine {
			return myTags[id]
		}
		names := make([]string, len(global[id]))
		for i, tc := range global[id] {
			names[i] = tc.Tag
		}
		return names
	}
	hasAll := func(have []string) bool {
		for _, want := range tags {
			found := false
			for _, h := range have {
				if h == want {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
		return true
	}

	categories := map[string]int{}
	facetTags := map[string]int{}
	for _, it := range items {
		if !hasAll(scopeTags(it.ID)) {
			continue
		}
		categories[it.Category]++
		if category != "" && it.Category != category {
			continue
		}
		for _, t := range scopeTags(it.ID) {
			facetTags[t]++
		}
		out.Total++
		if q.Limit > 0 && len(out.Results) >= q.Limit {
			continue
		}
		r := core.GearResult{Item: it, Mine: mine[it.ID], MyTags: myTags[it.ID], Tags: global[it.ID]}
		if r.MyTags == nil {
			r.MyTags = []string{}
		}
		if r.Tags == nil {
			r.Tags = []core.TagCount{}
		}
		out.Results = append(out.Results, r)
	}
	out.TagFacets = core.SortTagCounts(facetTags, 30)
	for c, n := range categories {
		out.CategoryFacets = append(out.CategoryFacets, core.CategoryCount{Category: c, Count: n})
	}
	sort.Slice(out.CategoryFacets, func(i, j int) bool { return out.CategoryFacets[i].Category < out.CategoryFacets[j].Category })
	return out, nil
}

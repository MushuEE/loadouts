package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/gmccloskey/loadouts/backend/internal/core"
	"github.com/gmccloskey/loadouts/backend/internal/db"
)

// LoadoutService owns the marriage of Templates and Items: creating, editing, validating,
// publishing, discovering, and forking Loadouts.
type LoadoutService struct {
	store     db.Store
	items     *InventoryService
	templates *TemplateService
	community *CommunityService
}

func NewLoadoutService(store db.Store, items *InventoryService, templates *TemplateService, community *CommunityService) *LoadoutService {
	return &LoadoutService{store: store, items: items, templates: templates, community: community}
}

// CreateLoadoutRequest is the input for starting a new loadout from a template.
type CreateLoadoutRequest struct {
	Name            string              `json:"name"`
	Description     string              `json:"description"`
	TemplateID      string              `json:"template_id"`
	TemplateVersion int                 `json:"template_version"`
	CommunityID     string              `json:"community_id"`
	Visibility      core.Visibility     `json:"visibility"`
	CoverImageURL   string              `json:"cover_image_url"`
	Entries         []core.LoadoutEntry `json:"entries"`
}

// Create starts a draft loadout scaffolded by a template version.
func (s *LoadoutService) Create(ctx context.Context, actorProfileID string, req CreateLoadoutRequest) (core.LoadoutDetail, error) {
	if actorProfileID == "" {
		return core.LoadoutDetail{}, fmt.Errorf("%w: a profile is required to create a loadout", core.ErrForbidden)
	}
	if strings.TrimSpace(req.Name) == "" {
		return core.LoadoutDetail{}, fmt.Errorf("%w: name is required", core.ErrInvalid)
	}

	templateID := req.TemplateID
	if templateID == "" {
		templateID = core.FreeformTemplateID // The generic "just add items" scaffold.
	}
	detail, err := s.templates.Detail(ctx, templateID, req.TemplateVersion)
	if err != nil {
		return core.LoadoutDetail{}, err
	}

	if req.CommunityID != "" {
		if _, err := s.store.GetCommunity(ctx, req.CommunityID); err != nil {
			return core.LoadoutDetail{}, fmt.Errorf("%w: community %s", core.ErrNotFound, req.CommunityID)
		}
	}

	visibility := req.Visibility
	if visibility == "" {
		visibility = core.VisibilityPrivate
	}

	now := time.Now().UTC()
	loadout := core.Loadout{
		ID:              core.NewID("ldt"),
		Name:            req.Name,
		Description:     req.Description,
		OwnerProfileID:  actorProfileID,
		CommunityID:     req.CommunityID,
		TemplateID:      detail.Template.ID,
		TemplateVersion: detail.Version.Version,
		Visibility:      visibility,
		Status:          core.StatusDraft,
		CoverImageURL:   req.CoverImageURL,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := s.store.CreateLoadout(ctx, loadout); err != nil {
		return core.LoadoutDetail{}, err
	}

	if len(req.Entries) > 0 {
		if _, err := s.ReplaceEntries(ctx, actorProfileID, loadout.ID, req.Entries); err != nil {
			return core.LoadoutDetail{}, err
		}
	}

	return s.Detail(ctx, loadout.ID, actorProfileID)
}

// UpdateLoadoutRequest patches loadout metadata. Nil fields are left untouched.
type UpdateLoadoutRequest struct {
	Name          *string          `json:"name"`
	Description   *string          `json:"description"`
	Visibility    *core.Visibility `json:"visibility"`
	CommunityID   *string          `json:"community_id"`
	CoverImageURL *string          `json:"cover_image_url"`
}

func (s *LoadoutService) Update(ctx context.Context, actorProfileID, loadoutID string, req UpdateLoadoutRequest) (core.LoadoutDetail, error) {
	loadout, err := s.requireOwner(ctx, actorProfileID, loadoutID)
	if err != nil {
		return core.LoadoutDetail{}, err
	}

	if req.Name != nil {
		loadout.Name = *req.Name
	}
	if req.Description != nil {
		loadout.Description = *req.Description
	}
	if req.Visibility != nil {
		loadout.Visibility = *req.Visibility
	}
	if req.CoverImageURL != nil {
		loadout.CoverImageURL = *req.CoverImageURL
	}
	if req.CommunityID != nil {
		if *req.CommunityID != "" {
			if _, err := s.store.GetCommunity(ctx, *req.CommunityID); err != nil {
				return core.LoadoutDetail{}, fmt.Errorf("%w: community %s", core.ErrNotFound, *req.CommunityID)
			}
		}
		loadout.CommunityID = *req.CommunityID
	}

	loadout.UpdatedAt = time.Now().UTC()
	if err := s.store.UpdateLoadout(ctx, loadout); err != nil {
		return core.LoadoutDetail{}, err
	}
	return s.Detail(ctx, loadout.ID, actorProfileID)
}

// ReplaceEntries swaps the whole entry tree. The editor always owns the full state, so a
// wholesale replace keeps the client simple and the write atomic.
func (s *LoadoutService) ReplaceEntries(ctx context.Context, actorProfileID, loadoutID string, entries []core.LoadoutEntry) (core.LoadoutDetail, error) {
	loadout, err := s.requireOwner(ctx, actorProfileID, loadoutID)
	if err != nil {
		return core.LoadoutDetail{}, err
	}

	prepared := make([]core.LoadoutEntry, 0, len(entries))
	seen := map[string]bool{}
	for i, e := range entries {
		if e.ItemID == "" {
			return core.LoadoutDetail{}, fmt.Errorf("%w: entry %d has no item_id", core.ErrInvalid, i)
		}
		if _, err := s.store.GetItem(ctx, e.ItemID); err != nil {
			return core.LoadoutDetail{}, fmt.Errorf("%w: item %s", core.ErrNotFound, e.ItemID)
		}
		if e.ID == "" {
			e.ID = core.NewID("ent")
		}
		if seen[e.ID] {
			return core.LoadoutDetail{}, fmt.Errorf("%w: duplicate entry id %s", core.ErrInvalid, e.ID)
		}
		seen[e.ID] = true

		e.LoadoutID = loadoutID
		if e.Quantity <= 0 {
			e.Quantity = 1
		}
		if e.CreatedAt.IsZero() {
			e.CreatedAt = time.Now().UTC()
		}
		prepared = append(prepared, e)
	}

	// Reject parent references that point at entries not present in this payload, which
	// would otherwise silently orphan part of the tree.
	for _, e := range prepared {
		if e.ParentEntryID != "" && !seen[e.ParentEntryID] {
			return core.LoadoutDetail{}, fmt.Errorf("%w: entry %s references unknown parent %s", core.ErrInvalid, e.ID, e.ParentEntryID)
		}
	}

	if err := s.store.ReplaceLoadoutEntries(ctx, loadoutID, prepared); err != nil {
		return core.LoadoutDetail{}, err
	}

	loadout.UpdatedAt = time.Now().UTC()
	if err := s.store.UpdateLoadout(ctx, loadout); err != nil {
		return core.LoadoutDetail{}, err
	}
	return s.Detail(ctx, loadoutID, actorProfileID)
}

// Publish enforces hard validation, then shares the loadout.
func (s *LoadoutService) Publish(ctx context.Context, actorProfileID, loadoutID string, visibility core.Visibility, communityID string) (core.LoadoutDetail, error) {
	loadout, err := s.requireOwner(ctx, actorProfileID, loadoutID)
	if err != nil {
		return core.LoadoutDetail{}, err
	}

	detail, err := s.Detail(ctx, loadoutID, actorProfileID)
	if err != nil {
		return core.LoadoutDetail{}, err
	}
	for _, issue := range detail.Issues {
		if issue.Severity == "error" {
			return core.LoadoutDetail{}, fmt.Errorf("%w: cannot publish, %s", core.ErrInvalid, issue.Message)
		}
	}

	if communityID != "" {
		if _, err := s.store.GetCommunity(ctx, communityID); err != nil {
			return core.LoadoutDetail{}, fmt.Errorf("%w: community %s", core.ErrNotFound, communityID)
		}
		loadout.CommunityID = communityID
	}
	if visibility == "" {
		visibility = core.VisibilityPublic
	}

	loadout.Visibility = visibility
	loadout.Status = core.StatusPublished
	loadout.UpdatedAt = time.Now().UTC()
	if err := s.store.UpdateLoadout(ctx, loadout); err != nil {
		return core.LoadoutDetail{}, err
	}
	return s.Detail(ctx, loadoutID, actorProfileID)
}

// Fork copies a visible loadout (and its whole entry tree) into the actor's account as a
// private draft. This is the "discover then remix" loop.
func (s *LoadoutService) Fork(ctx context.Context, actorProfileID, loadoutID string) (core.LoadoutDetail, error) {
	if actorProfileID == "" {
		return core.LoadoutDetail{}, fmt.Errorf("%w: a profile is required to fork", core.ErrForbidden)
	}
	source, err := s.store.GetLoadout(ctx, loadoutID)
	if err != nil {
		return core.LoadoutDetail{}, fmt.Errorf("%w: loadout %s", core.ErrNotFound, loadoutID)
	}
	if !source.IsVisibleTo(actorProfileID) {
		return core.LoadoutDetail{}, fmt.Errorf("%w: loadout %s is private", core.ErrForbidden, loadoutID)
	}

	entries, err := s.store.ListLoadoutEntries(ctx, loadoutID)
	if err != nil {
		return core.LoadoutDetail{}, err
	}

	now := time.Now().UTC()
	fork := core.Loadout{
		ID:              core.NewID("ldt"),
		Name:            source.Name + " (fork)",
		Description:     source.Description,
		OwnerProfileID:  actorProfileID,
		CommunityID:     source.CommunityID,
		TemplateID:      source.TemplateID,
		TemplateVersion: source.TemplateVersion, // Pin the same version so the copy behaves identically.
		Visibility:      core.VisibilityPrivate,
		Status:          core.StatusDraft,
		ForkedFrom:      source.ID,
		CoverImageURL:   source.CoverImageURL,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := s.store.CreateLoadout(ctx, fork); err != nil {
		return core.LoadoutDetail{}, err
	}

	// Remap entry IDs while preserving the parent/child structure.
	idMap := make(map[string]string, len(entries))
	for _, e := range entries {
		idMap[e.ID] = core.NewID("ent")
	}
	copied := make([]core.LoadoutEntry, 0, len(entries))
	for _, e := range entries {
		e.ID = idMap[e.ID]
		if e.ParentEntryID != "" {
			e.ParentEntryID = idMap[e.ParentEntryID]
		}
		e.LoadoutID = fork.ID
		e.CreatedAt = now
		copied = append(copied, e)
	}
	if err := s.store.ReplaceLoadoutEntries(ctx, fork.ID, copied); err != nil {
		return core.LoadoutDetail{}, err
	}

	source.ForkCount++
	if err := s.store.UpdateLoadout(ctx, source); err != nil {
		return core.LoadoutDetail{}, err
	}

	return s.Detail(ctx, fork.ID, actorProfileID)
}

func (s *LoadoutService) Delete(ctx context.Context, actorProfileID, loadoutID string) error {
	if _, err := s.requireOwner(ctx, actorProfileID, loadoutID); err != nil {
		return err
	}
	return s.store.DeleteLoadout(ctx, loadoutID)
}

// Detail assembles the full read model: resolved entry tree, stats, and validation issues.
func (s *LoadoutService) Detail(ctx context.Context, loadoutID, viewerProfileID string) (core.LoadoutDetail, error) {
	loadout, err := s.store.GetLoadout(ctx, loadoutID)
	if err != nil {
		return core.LoadoutDetail{}, fmt.Errorf("%w: loadout %s", core.ErrNotFound, loadoutID)
	}
	if !loadout.IsVisibleTo(viewerProfileID) {
		return core.LoadoutDetail{}, fmt.Errorf("%w: loadout %s is private", core.ErrForbidden, loadoutID)
	}

	tmpl, err := s.templates.Detail(ctx, loadout.TemplateID, loadout.TemplateVersion)
	if err != nil {
		return core.LoadoutDetail{}, err
	}
	owner, err := s.store.GetProfile(ctx, loadout.OwnerProfileID)
	if err != nil {
		owner = core.Profile{ID: loadout.OwnerProfileID, Handle: "unknown"}
	}
	entries, err := s.store.ListLoadoutEntries(ctx, loadoutID)
	if err != nil {
		return core.LoadoutDetail{}, err
	}

	// Items are resolved through the *owner's* layers, in the loadout's community context,
	// but private data only surfaces when the viewer is that owner.
	lctx := core.LayerContext{
		ViewerProfileID: viewerProfileID,
		OwnerProfileID:  loadout.OwnerProfileID,
		CommunityID:     loadout.CommunityID,
	}
	itemIDs := make([]string, 0, len(entries))
	for _, e := range entries {
		itemIDs = append(itemIDs, e.ItemID)
	}
	resolved := s.items.ResolveItems(ctx, itemIDs, lctx)

	tree := buildEntryTree(entries, resolved)
	stats := computeStats(entries, resolved)
	issues := validateEntries(tmpl.Version, entries, resolved)

	return core.LoadoutDetail{
		Loadout:  loadout,
		Owner:    owner,
		Template: tmpl,
		Entries:  tree,
		Stats:    stats,
		Issues:   issues,
	}, nil
}

// Discover lists loadouts for the public feed, a community feed, or a profile's shelf.
// Private loadouts are filtered out unless the viewer owns them.
func (s *LoadoutService) Discover(ctx context.Context, q core.DiscoverQuery, viewerProfileID string) ([]core.LoadoutSummary, error) {
	loadouts, err := s.store.ListLoadouts(ctx, q)
	if err != nil {
		return nil, err
	}

	summaries := make([]core.LoadoutSummary, 0, len(loadouts))
	for _, l := range loadouts {
		if !l.IsVisibleTo(viewerProfileID) {
			continue
		}

		entries, err := s.store.ListLoadoutEntries(ctx, l.ID)
		if err != nil {
			continue
		}
		lctx := core.LayerContext{
			ViewerProfileID: viewerProfileID,
			OwnerProfileID:  l.OwnerProfileID,
			CommunityID:     l.CommunityID,
		}
		itemIDs := make([]string, 0, len(entries))
		for _, e := range entries {
			itemIDs = append(itemIDs, e.ItemID)
		}
		resolved := s.items.ResolveItems(ctx, itemIDs, lctx)

		summary := core.LoadoutSummary{
			Loadout:     l,
			CommunityID: l.CommunityID,
			Stats:       computeStats(entries, resolved),
			ItemPreview: previewNames(entries, resolved, 4),
		}
		if owner, err := s.store.GetProfile(ctx, l.OwnerProfileID); err == nil {
			summary.OwnerHandle = owner.Handle
			summary.OwnerName = owner.DisplayName
		}
		if tmpl, err := s.store.GetTemplate(ctx, l.TemplateID); err == nil {
			summary.TemplateName = tmpl.Name
		}
		summaries = append(summaries, summary)
	}
	return summaries, nil
}

func (s *LoadoutService) requireOwner(ctx context.Context, actorProfileID, loadoutID string) (core.Loadout, error) {
	loadout, err := s.store.GetLoadout(ctx, loadoutID)
	if err != nil {
		return core.Loadout{}, fmt.Errorf("%w: loadout %s", core.ErrNotFound, loadoutID)
	}
	if actorProfileID == "" || loadout.OwnerProfileID != actorProfileID {
		return core.Loadout{}, fmt.Errorf("%w: only the owner may modify this loadout", core.ErrForbidden)
	}
	return loadout, nil
}

// --- Pure helpers (easy to unit test) ---

// buildEntryTree nests entries under their parents so the client can render the
// telescoping view directly.
func buildEntryTree(entries []core.LoadoutEntry, resolved map[string]core.ResolvedItem) []core.ResolvedEntry {
	byParent := map[string][]core.LoadoutEntry{}
	for _, e := range entries {
		byParent[e.ParentEntryID] = append(byParent[e.ParentEntryID], e)
	}
	for k := range byParent {
		list := byParent[k]
		sort.Slice(list, func(i, j int) bool { return list[i].Position < list[j].Position })
		byParent[k] = list
	}

	var build func(parentID string, depth int) []core.ResolvedEntry
	build = func(parentID string, depth int) []core.ResolvedEntry {
		if depth > 8 { // Defensive: never recurse forever on a corrupted parent chain.
			return nil
		}
		out := make([]core.ResolvedEntry, 0, len(byParent[parentID]))
		for _, e := range byParent[parentID] {
			out = append(out, core.ResolvedEntry{
				Entry:    e,
				Item:     resolved[e.ItemID],
				Children: build(e.ID, depth+1),
			})
		}
		return out
	}
	return build("", 0)
}

// computeStats rolls up weight and cost across every entry (nesting included, since the
// flat list already contains all descendants). Consumables are excluded from base weight.
func computeStats(entries []core.LoadoutEntry, resolved map[string]core.ResolvedItem) core.LoadoutStats {
	stats := core.LoadoutStats{}
	for _, e := range entries {
		item, ok := resolved[e.ItemID]
		if !ok {
			continue
		}
		qty := float64(e.Quantity)
		if qty <= 0 {
			qty = 1
		}

		weight := item.WeightG() * qty
		stats.TotalWeightG += weight
		stats.TotalCostCents += item.CostCents() * qty
		stats.ItemCount += int(qty)

		if item.IsConsumable() {
			stats.ConsumableWeightG += weight
		} else {
			stats.BaseWeightG += weight
		}
	}
	return stats
}

// validateEntries checks a loadout against its pinned template version and against the
// slots provided by container items. Missing required slots are warnings while drafting
// and become blocking errors at publish time (see Publish).
func validateEntries(version core.TemplateVersion, entries []core.LoadoutEntry, resolved map[string]core.ResolvedItem) []core.ValidationIssue {
	issues := []core.ValidationIssue{}
	entriesByID := map[string]core.LoadoutEntry{}
	for _, e := range entries {
		entriesByID[e.ID] = e
	}

	usage := map[string]int{} // "parentID|slotID" -> count

	for _, e := range entries {
		item, hasItem := resolved[e.ItemID]
		if !hasItem {
			issues = append(issues, core.ValidationIssue{
				SlotID: e.SlotID, EntryID: e.ID, Severity: "error",
				Message: fmt.Sprintf("item %s could not be resolved", e.ItemID),
			})
			continue
		}

		var slot core.SlotDefinition
		var found bool
		if e.ParentEntryID == "" {
			slot, found = version.SlotByID(e.SlotID)
			if !found && len(version.Slots) == 0 {
				continue // Freeform template: any slot id is fine.
			}
		} else {
			parent, ok := entriesByID[e.ParentEntryID]
			if !ok {
				issues = append(issues, core.ValidationIssue{
					SlotID: e.SlotID, EntryID: e.ID, Severity: "error",
					Message: fmt.Sprintf("entry %s has a dangling parent", e.ID),
				})
				continue
			}
			parentItem, ok := resolved[parent.ItemID]
			if !ok {
				continue
			}
			for _, ps := range parentItem.ProvidedSlots {
				if ps.ID == e.SlotID {
					slot, found = ps, true
					break
				}
			}
			if !found && len(parentItem.ProvidedSlots) == 0 {
				issues = append(issues, core.ValidationIssue{
					SlotID: e.SlotID, EntryID: e.ID, Severity: "warning",
					Message: fmt.Sprintf("%s is not a container but holds %s", parentItem.Name, item.Name),
				})
				continue
			}
		}

		if !found {
			issues = append(issues, core.ValidationIssue{
				SlotID: e.SlotID, EntryID: e.ID, Severity: "warning",
				Message: fmt.Sprintf("slot %q is not defined here", e.SlotID),
			})
			continue
		}

		if !slot.Accepts(item.Category) {
			issues = append(issues, core.ValidationIssue{
				SlotID: e.SlotID, EntryID: e.ID, Severity: "error",
				Message: fmt.Sprintf("%s (%s) is not accepted by slot %q", item.Name, item.Category, slot.Name),
			})
		}

		usageKey := e.ParentEntryID + "|" + e.SlotID
		usage[usageKey]++
		if capacity := slot.Capacity(); capacity > 0 && usage[usageKey] > capacity {
			issues = append(issues, core.ValidationIssue{
				SlotID: e.SlotID, EntryID: e.ID, Severity: "error",
				Message: fmt.Sprintf("slot %q holds at most %d item(s)", slot.Name, capacity),
			})
		}
	}

	// Required template slots must be filled.
	filled := map[string]bool{}
	for _, e := range entries {
		if e.ParentEntryID == "" {
			filled[e.SlotID] = true
		}
	}
	for _, slot := range version.Slots {
		if slot.Required && !filled[slot.ID] {
			issues = append(issues, core.ValidationIssue{
				SlotID: slot.ID, Severity: "error",
				Message: fmt.Sprintf("required slot %q is empty", slot.Name),
			})
		}
	}

	return issues
}

func previewNames(entries []core.LoadoutEntry, resolved map[string]core.ResolvedItem, limit int) []string {
	names := make([]string, 0, limit)
	for _, e := range entries {
		if item, ok := resolved[e.ItemID]; ok {
			names = append(names, item.Name)
		}
		if len(names) >= limit {
			break
		}
	}
	return names
}

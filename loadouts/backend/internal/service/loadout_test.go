package service

import (
	"context"
	"errors"
	"testing"

	"github.com/gmccloskey/loadouts/backend/internal/core"
)

// backpackingTemplate builds a small structured template for loadout tests.
func backpackingTemplate(t *testing.T, h *harness, author core.Profile) core.TemplateDetail {
	t.Helper()
	detail, err := h.templates.Create(context.Background(), author.ID, core.Template{Name: "Backpacking"}, core.SlotList{
		{ID: "shelter", Name: "Shelter", AcceptedCategories: []string{"shelter"}, Required: true},
		{ID: "pack", Name: "Pack", AcceptedCategories: []string{"pack"}},
	}, "v1")
	if err != nil {
		t.Fatalf("create template: %v", err)
	}
	return detail
}

func TestLoadout_LifecycleAndNestedStats(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	author := h.profile(t, "author")
	tmpl := backpackingTemplate(t, h, author)

	h.item(t, "tent", "shelter", 800, 24000, false, nil)
	h.item(t, "pack", "pack", 900, 37500, false, core.SlotList{
		{ID: "main", Name: "Main", AcceptedCategories: []string{"universal"}, MaxItems: -1},
	})
	h.item(t, "food", "consumable", 700, 1500, true, nil)

	detail, err := h.loadouts.Create(ctx, author.ID, CreateLoadoutRequest{
		Name: "Weekender", TemplateID: tmpl.Template.ID,
	})
	if err != nil {
		t.Fatalf("create loadout: %v", err)
	}
	if detail.Loadout.Status != core.StatusDraft || detail.Loadout.Visibility != core.VisibilityPrivate {
		t.Errorf("new loadouts should start as private drafts, got %s/%s", detail.Loadout.Status, detail.Loadout.Visibility)
	}

	// A required slot is empty, so publishing must be blocked.
	if _, err := h.loadouts.Publish(ctx, author.ID, detail.Loadout.ID, core.VisibilityPublic, ""); !errors.Is(err, core.ErrInvalid) {
		t.Errorf("publishing with an empty required slot should fail, got %v", err)
	}

	packEntry := "ent_pack"
	detail, err = h.loadouts.ReplaceEntries(ctx, author.ID, detail.Loadout.ID, []core.LoadoutEntry{
		{SlotID: "shelter", ItemID: "tent"},
		{ID: packEntry, SlotID: "pack", ItemID: "pack"},
		// Nested two days of food inside the pack.
		{SlotID: "main", ParentEntryID: packEntry, ItemID: "food", Quantity: 2},
	})
	if err != nil {
		t.Fatalf("replace entries: %v", err)
	}

	if len(detail.Entries) != 2 {
		t.Fatalf("expected 2 root entries, got %d", len(detail.Entries))
	}
	var packNode core.ResolvedEntry
	for _, e := range detail.Entries {
		if e.Entry.ID == packEntry {
			packNode = e
		}
	}
	if len(packNode.Children) != 1 {
		t.Fatalf("food should be nested under the pack, got %d children", len(packNode.Children))
	}

	// 800 + 900 + (700 * 2 consumable)
	if detail.Stats.TotalWeightG != 3100 {
		t.Errorf("total weight = %v, want 3100", detail.Stats.TotalWeightG)
	}
	if detail.Stats.BaseWeightG != 1700 {
		t.Errorf("base weight should exclude consumables: got %v, want 1700", detail.Stats.BaseWeightG)
	}
	if detail.Stats.ConsumableWeightG != 1400 {
		t.Errorf("consumable weight = %v, want 1400", detail.Stats.ConsumableWeightG)
	}
	if len(detail.Issues) != 0 {
		t.Errorf("expected no validation issues, got %+v", detail.Issues)
	}

	published, err := h.loadouts.Publish(ctx, author.ID, detail.Loadout.ID, core.VisibilityPublic, "")
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	if published.Loadout.Status != core.StatusPublished {
		t.Errorf("status = %s, want published", published.Loadout.Status)
	}
}

func TestLoadout_RejectsWrongCategoryAndForeignEdits(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	author := h.profile(t, "author")
	stranger := h.profile(t, "stranger")
	tmpl := backpackingTemplate(t, h, author)

	h.item(t, "tent", "shelter", 800, 24000, false, nil)
	h.item(t, "boots", "shoes", 600, 14000, false, nil)

	detail, err := h.loadouts.Create(ctx, author.ID, CreateLoadoutRequest{Name: "Mixed", TemplateID: tmpl.Template.ID})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	detail, err = h.loadouts.ReplaceEntries(ctx, author.ID, detail.Loadout.ID, []core.LoadoutEntry{
		{SlotID: "shelter", ItemID: "boots"}, // Wrong category for the slot.
	})
	if err != nil {
		t.Fatalf("replace entries: %v", err)
	}

	var sawCategoryError bool
	for _, issue := range detail.Issues {
		if issue.SlotID == "shelter" && issue.Severity == "error" {
			sawCategoryError = true
		}
	}
	if !sawCategoryError {
		t.Errorf("expected a category validation error, got %+v", detail.Issues)
	}

	if _, err := h.loadouts.ReplaceEntries(ctx, stranger.ID, detail.Loadout.ID, nil); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("non-owners must not edit a loadout, got %v", err)
	}
	if _, err := h.loadouts.Detail(ctx, detail.Loadout.ID, stranger.ID); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("private loadouts must be hidden from other profiles, got %v", err)
	}
}

func TestLoadout_ForkCopiesNestedTreeAndPinsVersion(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	author := h.profile(t, "author")
	forker := h.profile(t, "forker")
	tmpl := backpackingTemplate(t, h, author)

	h.item(t, "tent", "shelter", 800, 24000, false, nil)
	h.item(t, "pack", "pack", 900, 37500, false, core.SlotList{
		{ID: "main", Name: "Main", AcceptedCategories: []string{"universal"}, MaxItems: -1},
	})
	h.item(t, "food", "consumable", 700, 1500, true, nil)

	detail, _ := h.loadouts.Create(ctx, author.ID, CreateLoadoutRequest{Name: "Original", TemplateID: tmpl.Template.ID})
	packEntry := "ent_pack"
	if _, err := h.loadouts.ReplaceEntries(ctx, author.ID, detail.Loadout.ID, []core.LoadoutEntry{
		{SlotID: "shelter", ItemID: "tent"},
		{ID: packEntry, SlotID: "pack", ItemID: "pack"},
		{SlotID: "main", ParentEntryID: packEntry, ItemID: "food"},
	}); err != nil {
		t.Fatalf("replace entries: %v", err)
	}
	if _, err := h.loadouts.Publish(ctx, author.ID, detail.Loadout.ID, core.VisibilityPublic, ""); err != nil {
		t.Fatalf("publish: %v", err)
	}

	// The template evolves after publication; the fork must still pin v1.
	if _, err := h.templates.PublishVersion(ctx, author.ID, tmpl.Template.ID, core.SlotList{
		{ID: "shelter", Name: "Shelter", AcceptedCategories: []string{"shelter"}, Required: true},
		{ID: "bear-can", Name: "Bear Canister", AcceptedCategories: []string{"organizer"}, Required: true},
	}, "v2"); err != nil {
		t.Fatalf("publish v2: %v", err)
	}

	fork, err := h.loadouts.Fork(ctx, forker.ID, detail.Loadout.ID)
	if err != nil {
		t.Fatalf("fork: %v", err)
	}

	if fork.Loadout.OwnerProfileID != forker.ID {
		t.Errorf("fork owner = %s, want %s", fork.Loadout.OwnerProfileID, forker.ID)
	}
	if fork.Loadout.ForkedFrom != detail.Loadout.ID {
		t.Errorf("fork should record its source")
	}
	if fork.Loadout.Visibility != core.VisibilityPrivate || fork.Loadout.Status != core.StatusDraft {
		t.Errorf("forks should land as private drafts, got %s/%s", fork.Loadout.Visibility, fork.Loadout.Status)
	}
	if fork.Loadout.TemplateVersion != 1 {
		t.Errorf("fork should stay pinned to v1, got v%d", fork.Loadout.TemplateVersion)
	}
	if len(fork.Issues) != 0 {
		t.Errorf("pinned v1 fork should validate cleanly, got %+v", fork.Issues)
	}

	// The nested structure survives with fresh entry IDs.
	var forkedPack core.ResolvedEntry
	for _, e := range fork.Entries {
		if e.Item.ID == "pack" {
			forkedPack = e
		}
	}
	if len(forkedPack.Children) != 1 {
		t.Fatalf("nested child lost in fork, got %d children", len(forkedPack.Children))
	}
	if forkedPack.Entry.ID == packEntry {
		t.Error("forked entries should get fresh IDs")
	}

	source, _ := h.loadouts.Detail(ctx, detail.Loadout.ID, author.ID)
	if source.Loadout.ForkCount != 1 {
		t.Errorf("source fork_count = %d, want 1", source.Loadout.ForkCount)
	}
}

func TestLoadout_DiscoverRespectsVisibility(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	author := h.profile(t, "author")
	viewer := h.profile(t, "viewer")
	h.item(t, "tent", "shelter", 800, 24000, false, nil)

	// One public, one private, both on the freeform template.
	pub, _ := h.loadouts.Create(ctx, author.ID, CreateLoadoutRequest{
		Name: "Public Kit", TemplateID: core.FreeformTemplateID,
		Entries: []core.LoadoutEntry{{SlotID: "free-0", ItemID: "tent"}},
	})
	if _, err := h.loadouts.Publish(ctx, author.ID, pub.Loadout.ID, core.VisibilityPublic, ""); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if _, err := h.loadouts.Create(ctx, author.ID, CreateLoadoutRequest{
		Name: "Secret Kit", TemplateID: core.FreeformTemplateID,
	}); err != nil {
		t.Fatalf("create private: %v", err)
	}

	feed, err := h.loadouts.Discover(ctx, core.DiscoverQuery{OnlyPublic: true}, viewer.ID)
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if len(feed) != 1 || feed[0].Loadout.Name != "Public Kit" {
		t.Fatalf("discover should only surface public loadouts, got %+v", feed)
	}
	if feed[0].OwnerHandle != "author" {
		t.Errorf("summary should carry the owner handle, got %q", feed[0].OwnerHandle)
	}

	// The owner's own listing includes drafts.
	own, err := h.loadouts.Discover(ctx, core.DiscoverQuery{ProfileID: author.ID}, author.ID)
	if err != nil {
		t.Fatalf("own listing: %v", err)
	}
	if len(own) != 2 {
		t.Errorf("owner should see both loadouts, got %d", len(own))
	}
}

func TestLoadout_RejectsDanglingParentReference(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	author := h.profile(t, "author")
	h.item(t, "tent", "shelter", 800, 24000, false, nil)

	detail, _ := h.loadouts.Create(ctx, author.ID, CreateLoadoutRequest{Name: "Broken", TemplateID: core.FreeformTemplateID})
	_, err := h.loadouts.ReplaceEntries(ctx, author.ID, detail.Loadout.ID, []core.LoadoutEntry{
		{SlotID: "free-0", ParentEntryID: "does-not-exist", ItemID: "tent"},
	})
	if !errors.Is(err, core.ErrInvalid) {
		t.Errorf("dangling parent reference should be rejected, got %v", err)
	}
}

func TestInventory_ResolveItemAppliesCommunityLayer(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	admin := h.profile(t, "admin")
	h.item(t, "tent", "shelter", 800, 24000, false, nil)

	community, err := h.community.Create(ctx, admin.ID, core.Community{Name: "UL"})
	if err != nil {
		t.Fatalf("create community: %v", err)
	}
	if _, err := h.community.SetItemLayer(ctx, admin.ID, community.ID, "tent", core.Metadata{
		"ul": map[string]interface{}{"ul_score": 9.1},
	}); err != nil {
		t.Fatalf("set layer: %v", err)
	}

	inScope, err := h.inventory.ResolveItem(ctx, "tent", core.LayerContext{CommunityID: community.ID})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if _, ok := inScope.Metadata["ul"]; !ok {
		t.Errorf("community layer should apply in scope, got %+v", inScope.Metadata)
	}

	outOfScope, _ := h.inventory.ResolveItem(ctx, "tent", core.LayerContext{})
	if _, leaked := outOfScope.Metadata["ul"]; leaked {
		t.Errorf("community layer leaked outside its scope: %+v", outOfScope.Metadata)
	}
}

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/gmccloskey/loadouts/backend/internal/core"
)

// Custom slots are the owner's answer to "my kit has something the template's author never
// imagined". They live on the loadout, so the tests below are mostly about that boundary:
// the template must stay untouched, and the loadout must behave as though the slot were
// always part of it.

func TestLoadout_AddCustomSlotAcceptsItems(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	owner := h.profile(t, "owner")
	tmpl := backpackingTemplate(t, h, owner)

	h.item(t, "tent", "shelter", 800, 24000, false, nil)
	h.item(t, "radio", "electronics", 120, 9000, false, nil)

	detail, err := h.loadouts.Create(ctx, owner.ID, CreateLoadoutRequest{Name: "Trip", TemplateID: tmpl.Template.ID})
	if err != nil {
		t.Fatalf("create loadout: %v", err)
	}
	id := detail.Loadout.ID

	detail, err = h.loadouts.AddSlot(ctx, owner.ID, id, core.SlotDefinition{
		ID: "comms", Name: "Comms", AcceptedCategories: []string{"electronics"},
	})
	if err != nil {
		t.Fatalf("add slot: %v", err)
	}
	if len(detail.Loadout.ExtraSlots) != 1 {
		t.Fatalf("expected 1 custom slot, got %d", len(detail.Loadout.ExtraSlots))
	}

	// The template is shared. A personal slot must not leak into it, or one person's
	// tweak would silently reshape everyone else's loadout.
	fresh, err := h.templates.Detail(ctx, tmpl.Template.ID, tmpl.Version.Version)
	if err != nil {
		t.Fatalf("reload template: %v", err)
	}
	if _, found := fresh.Version.Slots.ByID("comms"); found {
		t.Error("a loadout's custom slot leaked into the shared template")
	}

	// The whole point: the slot has to actually hold gear, with no validation complaints.
	detail, err = h.loadouts.ReplaceEntries(ctx, owner.ID, id, []core.LoadoutEntry{
		{ID: core.NewID("ent"), SlotID: "shelter", ItemID: "tent", Quantity: 1},
		{ID: core.NewID("ent"), SlotID: "comms", ItemID: "radio", Quantity: 1},
	})
	if err != nil {
		t.Fatalf("replace entries: %v", err)
	}
	for _, issue := range detail.Issues {
		if issue.SlotID == "comms" {
			t.Errorf("custom slot should validate like any other, got %q", issue.Message)
		}
	}
	if detail.Stats.ItemCount != 2 {
		t.Errorf("item count = %d, want 2", detail.Stats.ItemCount)
	}
}

func TestLoadout_CustomSlotEnforcesItsOwnRules(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	owner := h.profile(t, "owner")
	tmpl := backpackingTemplate(t, h, owner)

	h.item(t, "radio", "electronics", 120, 9000, false, nil)
	h.item(t, "tent", "shelter", 800, 24000, false, nil)

	detail, _ := h.loadouts.Create(ctx, owner.ID, CreateLoadoutRequest{Name: "Trip", TemplateID: tmpl.Template.ID})
	id := detail.Loadout.ID

	if _, err := h.loadouts.AddSlot(ctx, owner.ID, id, core.SlotDefinition{
		ID: "comms", Name: "Comms", AcceptedCategories: []string{"electronics"}, MaxItems: 1,
	}); err != nil {
		t.Fatalf("add slot: %v", err)
	}

	// A custom slot is a real slot: it rejects the wrong category rather than accepting
	// anything simply because the owner invented it.
	detail, err := h.loadouts.ReplaceEntries(ctx, owner.ID, id, []core.LoadoutEntry{
		{ID: core.NewID("ent"), SlotID: "comms", ItemID: "tent", Quantity: 1},
	})
	if err != nil {
		t.Fatalf("replace entries: %v", err)
	}
	var rejected bool
	for _, issue := range detail.Issues {
		if issue.SlotID == "comms" && issue.Severity == "error" {
			rejected = true
		}
	}
	if !rejected {
		t.Error("a shelter in an electronics-only custom slot should be an error")
	}
}

func TestLoadout_AddSlotIsOwnerOnlyAndRejectsCollisions(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	owner := h.profile(t, "owner")
	stranger := h.profile(t, "stranger")
	tmpl := backpackingTemplate(t, h, owner)

	detail, _ := h.loadouts.Create(ctx, owner.ID, CreateLoadoutRequest{Name: "Trip", TemplateID: tmpl.Template.ID})
	id := detail.Loadout.ID

	if _, err := h.loadouts.AddSlot(ctx, stranger.ID, id, core.SlotDefinition{Name: "Theirs"}); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("a non-owner adding a slot should be forbidden, got %v", err)
	}

	if _, err := h.loadouts.AddSlot(ctx, owner.ID, id, core.SlotDefinition{Name: "   "}); !errors.Is(err, core.ErrInvalid) {
		t.Errorf("a blank name should be invalid, got %v", err)
	}

	// Colliding with a template slot must fail loudly. EffectiveSlots lets the template
	// win, so a silent accept would create a slot that can never be filled.
	if _, err := h.loadouts.AddSlot(ctx, owner.ID, id, core.SlotDefinition{ID: "shelter", Name: "Mine"}); !errors.Is(err, core.ErrInvalid) {
		t.Errorf("colliding with a template slot should be invalid, got %v", err)
	}
}

func TestLoadout_RemoveSlotTakesItsContentsAndSparesTemplateSlots(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	owner := h.profile(t, "owner")
	tmpl := backpackingTemplate(t, h, owner)

	h.item(t, "tent", "shelter", 800, 24000, false, nil)
	h.item(t, "radio", "electronics", 120, 9000, false, nil)

	detail, _ := h.loadouts.Create(ctx, owner.ID, CreateLoadoutRequest{Name: "Trip", TemplateID: tmpl.Template.ID})
	id := detail.Loadout.ID
	if _, err := h.loadouts.AddSlot(ctx, owner.ID, id, core.SlotDefinition{
		ID: "comms", Name: "Comms", AcceptedCategories: []string{"electronics"},
	}); err != nil {
		t.Fatalf("add slot: %v", err)
	}
	if _, err := h.loadouts.ReplaceEntries(ctx, owner.ID, id, []core.LoadoutEntry{
		{ID: core.NewID("ent"), SlotID: "shelter", ItemID: "tent", Quantity: 1},
		{ID: core.NewID("ent"), SlotID: "comms", ItemID: "radio", Quantity: 1},
	}); err != nil {
		t.Fatalf("replace entries: %v", err)
	}

	// Template slots belong to the version this loadout is pinned to.
	if _, err := h.loadouts.RemoveSlot(ctx, owner.ID, id, "shelter"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("removing a template slot should not be possible, got %v", err)
	}

	after, err := h.loadouts.RemoveSlot(ctx, owner.ID, id, "comms")
	if err != nil {
		t.Fatalf("remove slot: %v", err)
	}
	if len(after.Loadout.ExtraSlots) != 0 {
		t.Errorf("custom slot survived removal: %+v", after.Loadout.ExtraSlots)
	}
	// The radio goes with it. Leaving it behind would orphan an entry into a slot that no
	// longer exists, which looks like corruption rather than a deletion.
	if after.Stats.ItemCount != 1 {
		t.Errorf("item count = %d, want 1 (the radio should have gone with its slot)", after.Stats.ItemCount)
	}
}

func TestLoadout_ForkCarriesCustomSlots(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	owner := h.profile(t, "owner")
	forker := h.profile(t, "forker")
	tmpl := backpackingTemplate(t, h, owner)

	h.item(t, "tent", "shelter", 800, 24000, false, nil)
	h.item(t, "radio", "electronics", 120, 9000, false, nil)

	detail, _ := h.loadouts.Create(ctx, owner.ID, CreateLoadoutRequest{Name: "Trip", TemplateID: tmpl.Template.ID})
	id := detail.Loadout.ID
	if _, err := h.loadouts.AddSlot(ctx, owner.ID, id, core.SlotDefinition{
		ID: "comms", Name: "Comms", AcceptedCategories: []string{"electronics"},
	}); err != nil {
		t.Fatalf("add slot: %v", err)
	}
	if _, err := h.loadouts.ReplaceEntries(ctx, owner.ID, id, []core.LoadoutEntry{
		{ID: core.NewID("ent"), SlotID: "shelter", ItemID: "tent", Quantity: 1},
		{ID: core.NewID("ent"), SlotID: "comms", ItemID: "radio", Quantity: 1},
	}); err != nil {
		t.Fatalf("replace entries: %v", err)
	}
	if _, err := h.loadouts.Publish(ctx, owner.ID, id, core.VisibilityPublic, ""); err != nil {
		t.Fatalf("publish: %v", err)
	}

	fork, err := h.loadouts.Fork(ctx, forker.ID, id)
	if err != nil {
		t.Fatalf("fork: %v", err)
	}
	if _, found := fork.Loadout.EffectiveSlots(fork.Template.Version).ByID("comms"); !found {
		t.Fatal("the fork lost the custom slot, so its copied entries have nowhere to live")
	}
	// Without the slot the radio would sit in an undefined slot and be flagged.
	for _, issue := range fork.Issues {
		if issue.SlotID == "comms" {
			t.Errorf("forked custom slot should validate cleanly, got %q", issue.Message)
		}
	}

	// The fork owns its slots outright: editing them must not touch the original.
	if _, err := h.loadouts.RemoveSlot(ctx, forker.ID, fork.Loadout.ID, "comms"); err != nil {
		t.Fatalf("remove slot on fork: %v", err)
	}
	source, err := h.loadouts.Detail(ctx, id, owner.ID)
	if err != nil {
		t.Fatalf("reload source: %v", err)
	}
	if _, found := source.Loadout.EffectiveSlots(source.Template.Version).ByID("comms"); !found {
		t.Error("removing a slot from the fork also removed it from the original")
	}
}

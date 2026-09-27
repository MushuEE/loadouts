package service

import (
	"context"
	"errors"
	"testing"

	"github.com/gmccloskey/loadouts/backend/internal/core"
)

func (h *harness) siteAdmin(t *testing.T, handle string) core.Profile {
	t.Helper()
	user, err := h.identity.CreateUser(context.Background(), handle+"@example.com", handle)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	profile, err := h.identity.CreateProfile(context.Background(), core.Profile{UserID: user.ID, Handle: handle, IsSiteAdmin: true})
	if err != nil {
		t.Fatalf("create profile: %v", err)
	}
	return profile
}

// hikerLayout puts a hiker on a 16x16 canvas with the shelter slot on its torso and the
// pack slot on a tile beside it.
func hikerLayout() *core.PaperdollLayout {
	return &core.PaperdollLayout{
		Cols: 16, Rows: 16,
		Blocks: []core.PaperdollBlock{
			{Block: "hiker"},
			{Block: core.TileBlock, Col: 12, Row: 0, Cols: 4, Rows: 4},
		},
		Bindings: []core.PaperdollBinding{
			{SlotID: "shelter", Cells: core.CellRuns{{3*16 + 3, 4}}},
			{SlotID: "pack", Cells: core.CellRuns{{12, 4}, {28, 4}, {44, 4}, {60, 4}}},
		},
	}
}

func tileLayout() *core.PaperdollLayout {
	l := hikerLayout()
	l.Blocks = l.Blocks[1:]
	l.Bindings = l.Bindings[1:]
	return l
}

func TestPaperdoll_FiguresAreForSiteAdmins(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	owner := h.profile(t, "owner")
	admin := h.siteAdmin(t, "admin")
	tmpl := backpackingTemplate(t, h, owner)

	if _, err := h.templates.PublishPaperdoll(ctx, owner.ID, tmpl.Template.ID, hikerLayout(), ""); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("a template owner should not place figures, got %v", err)
	}

	// Tiles have no artwork to curate, so the owner lays those out themselves.
	detail, err := h.templates.PublishPaperdoll(ctx, owner.ID, tmpl.Template.ID, tileLayout(), "")
	if err != nil {
		t.Fatalf("owner tile layout: %v", err)
	}
	if detail.Version.Version != 2 || detail.Version.Paperdoll == nil || detail.Version.Changelog != "Update paperdoll" {
		t.Errorf("unexpected version: %+v", detail.Version)
	}
	if len(detail.Version.Slots) != len(tmpl.Version.Slots) {
		t.Error("a paperdoll version must keep the slots it was drawn against")
	}

	// A site admin can lay out a figure on a template they do not own.
	detail, err = h.templates.PublishPaperdoll(ctx, admin.ID, tmpl.Template.ID, hikerLayout(), "hiker")
	if err != nil {
		t.Fatalf("admin figure layout: %v", err)
	}
	if !detail.Version.Paperdoll.HasFigures() {
		t.Error("the hiker did not make it into the version")
	}

	// And the earlier version is untouched.
	v1, err := h.templates.Detail(ctx, tmpl.Template.ID, 1)
	if err != nil {
		t.Fatalf("fetch v1: %v", err)
	}
	if v1.Version.Paperdoll != nil {
		t.Error("v1 grew a paperdoll")
	}

	removed, err := h.templates.PublishPaperdoll(ctx, owner.ID, tmpl.Template.ID, nil, "")
	if err != nil {
		t.Fatalf("remove layout: %v", err)
	}
	if removed.Version.Paperdoll != nil || removed.Version.Changelog != "Remove paperdoll" {
		t.Errorf("layout was not removed: %+v", removed.Version)
	}
}

func TestPaperdoll_StrangersAndPlatformAreRefused(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	owner := h.profile(t, "owner")
	stranger := h.profile(t, "stranger")
	admin := h.siteAdmin(t, "admin")
	tmpl := backpackingTemplate(t, h, owner)

	if _, err := h.templates.PublishPaperdoll(ctx, stranger.ID, tmpl.Template.ID, tileLayout(), ""); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("a stranger should not edit someone else's template, got %v", err)
	}
	if _, err := h.templates.PublishPaperdoll(ctx, admin.ID, core.FreeformTemplateID, nil, ""); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("platform templates stay immutable even for site admins, got %v", err)
	}
}

func TestPaperdoll_InvalidLayoutRejected(t *testing.T) {
	h := newHarness(t)
	owner := h.profile(t, "owner")
	tmpl := backpackingTemplate(t, h, owner)

	l := tileLayout()
	l.Bindings[0].SlotID = "no-such-slot"
	if _, err := h.templates.PublishPaperdoll(context.Background(), owner.ID, tmpl.Template.ID, l, ""); !errors.Is(err, core.ErrInvalid) {
		t.Errorf("want ErrInvalid, got %v", err)
	}
}

func TestPaperdoll_CarriedIntoTheNextSlotVersion(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	admin := h.siteAdmin(t, "admin")
	tmpl := backpackingTemplate(t, h, admin)
	if _, err := h.templates.PublishPaperdoll(ctx, admin.ID, tmpl.Template.ID, hikerLayout(), ""); err != nil {
		t.Fatalf("layout: %v", err)
	}

	// Dropping the pack slot keeps the figure and the shelter binding, and loses only the
	// binding that would now point at nothing.
	v3, err := h.templates.PublishVersion(ctx, admin.ID, tmpl.Template.ID, core.SlotList{
		{ID: "shelter", Name: "Shelter", AcceptedCategories: []string{"shelter"}, Required: true},
	}, "drop pack")
	if err != nil {
		t.Fatalf("publish v3: %v", err)
	}
	pd := v3.Version.Paperdoll
	if pd == nil || len(pd.Blocks) != 2 || len(pd.Bindings) != 1 || pd.Bindings[0].SlotID != "shelter" {
		t.Errorf("paperdoll not carried forward correctly: %+v", pd)
	}
}

func TestLoadout_MovesToANewerVersionOnlyWhenNothingIsStranded(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	owner := h.profile(t, "owner")
	tmpl := backpackingTemplate(t, h, owner)
	h.item(t, "tent", "shelter", 800, 24000, false, nil)
	h.item(t, "pack", "pack", 900, 37500, false, nil)

	detail, err := h.loadouts.Create(ctx, owner.ID, CreateLoadoutRequest{Name: "Trip", TemplateID: tmpl.Template.ID})
	if err != nil {
		t.Fatalf("create loadout: %v", err)
	}
	id := detail.Loadout.ID
	if _, err := h.loadouts.ReplaceEntries(ctx, owner.ID, id, []core.LoadoutEntry{
		{ID: core.NewID("ent"), SlotID: "shelter", ItemID: "tent", Quantity: 1},
		{ID: core.NewID("ent"), SlotID: "pack", ItemID: "pack", Quantity: 1},
	}); err != nil {
		t.Fatalf("fill: %v", err)
	}

	// v2 only adds a layout, so moving is safe.
	if _, err := h.templates.PublishPaperdoll(ctx, owner.ID, tmpl.Template.ID, tileLayout(), ""); err != nil {
		t.Fatalf("layout: %v", err)
	}
	two := 2
	moved, err := h.loadouts.Update(ctx, owner.ID, id, UpdateLoadoutRequest{TemplateVersion: &two})
	if err != nil {
		t.Fatalf("move to v2: %v", err)
	}
	if moved.Loadout.TemplateVersion != 2 || moved.Template.Version.Paperdoll == nil {
		t.Errorf("loadout did not pick up v2's paperdoll: v%d", moved.Loadout.TemplateVersion)
	}

	// v3 drops the pack slot this loadout uses, so the move would strand the pack.
	if _, err := h.templates.PublishVersion(ctx, owner.ID, tmpl.Template.ID, core.SlotList{
		{ID: "shelter", Name: "Shelter", AcceptedCategories: []string{"shelter"}, Required: true},
	}, "drop pack"); err != nil {
		t.Fatalf("publish v3: %v", err)
	}
	three := 3
	if _, err := h.loadouts.Update(ctx, owner.ID, id, UpdateLoadoutRequest{TemplateVersion: &three}); !errors.Is(err, core.ErrInvalid) {
		t.Errorf("a move that strands gear should be invalid, got %v", err)
	}
	stranger := h.profile(t, "stranger")
	if _, err := h.loadouts.Update(ctx, stranger.ID, id, UpdateLoadoutRequest{TemplateVersion: &two}); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("only the owner moves a loadout, got %v", err)
	}
}

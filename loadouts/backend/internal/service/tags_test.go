package service

import (
	"context"
	"reflect"
	"testing"

	"github.com/gmccloskey/loadouts/backend/internal/core"
)

func TestTags_FilterNarrowsStatsAndPluginsButNotTheTree(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	owner := h.profile(t, "owner")
	viewer := h.profile(t, "viewer")
	tmpl, err := h.templates.Create(ctx, owner.ID, core.Template{Name: "Layers", IsPublic: true}, core.SlotList{
		{ID: "torso", Name: "Torso", AcceptedCategories: []string{"shirt"}, MaxItems: 3},
		{ID: "shelter", Name: "Shelter", AcceptedCategories: []string{"shelter"}},
	}, "v1")
	if err != nil {
		t.Fatalf("create template: %v", err)
	}
	h.item(t, "tee", "shirt", 150, 0, false, nil)
	h.item(t, "fleece", "shirt", 300, 0, false, nil)
	h.item(t, "tent", "shelter", 800, 0, false, nil)

	if tags, err := h.inventory.SetItemTags(ctx, owner.ID, "tee", []string{"#WarmWear"}); err != nil || !reflect.DeepEqual(tags, []string{"warmwear"}) {
		t.Fatalf("tag tee: %v %v", tags, err)
	}
	if _, err := h.inventory.SetItemTags(ctx, owner.ID, "fleece", []string{"coldwear"}); err != nil {
		t.Fatalf("tag fleece: %v", err)
	}

	created, err := h.loadouts.Create(ctx, owner.ID, CreateLoadoutRequest{
		Name: "All seasons", TemplateID: tmpl.Template.ID,
		Entries: []core.LoadoutEntry{
			{SlotID: "torso", ItemID: "tee"},
			{SlotID: "torso", ItemID: "fleece"},
			{SlotID: "shelter", ItemID: "tent"},
		},
	})
	if err != nil {
		t.Fatalf("create loadout: %v", err)
	}
	if _, err := h.loadouts.Publish(ctx, owner.ID, created.Loadout.ID, core.VisibilityPublic, ""); err != nil {
		t.Fatalf("publish: %v", err)
	}

	// A viewer's own tags on the same gear are theirs alone. They must not leak into
	// someone else's loadout, which filters by its owner's tags.
	if _, err := h.inventory.SetItemTags(ctx, viewer.ID, "fleece", []string{"warmwear"}); err != nil {
		t.Fatalf("viewer tags fleece: %v", err)
	}

	// Tags are the owner's, so a viewer filters by the same ones the owner sees.
	detail, err := h.loadouts.DetailFiltered(ctx, created.Loadout.ID, viewer.ID, core.TagFilter{Tags: []string{"warmwear"}})
	if err != nil {
		t.Fatalf("detail: %v", err)
	}
	if len(detail.Entries) != 3 {
		t.Fatalf("hidden entries must stay in the tree, got %d entries", len(detail.Entries))
	}
	var hidden []string
	for _, e := range detail.Entries {
		if e.Hidden {
			hidden = append(hidden, e.Entry.ItemID)
		}
	}
	if !reflect.DeepEqual(hidden, []string{"fleece"}) {
		t.Errorf("hidden = %v, want [fleece]", hidden)
	}
	if detail.Stats.TotalWeightG != 950 {
		t.Errorf("filtered weight = %v, want 950 (tee + tent)", detail.Stats.TotalWeightG)
	}
	if detail.Filter.ShownEntries != 2 || detail.Filter.TotalEntries != 3 {
		t.Errorf("shown/total = %d/%d, want 2/3", detail.Filter.ShownEntries, detail.Filter.TotalEntries)
	}
	if len(detail.Filter.AvailableTags) != 2 {
		t.Errorf("available tags = %v, want both", detail.Filter.AvailableTags)
	}

	// Plugins see the same narrowed rows.
	data := loadoutWidgetData(detail, viewer.ID)
	if len(data.Rows) != 2 {
		t.Errorf("plugin rows = %d, want 2", len(data.Rows))
	}

	// Globally, tags aggregate over profiles: two people call the gear warmwear.
	global, err := h.inventory.PopularTags(ctx, "", 10)
	if err != nil {
		t.Fatalf("popular tags: %v", err)
	}
	if !reflect.DeepEqual(global, []core.TagCount{{Tag: "warmwear", Count: 2}, {Tag: "coldwear", Count: 1}}) {
		t.Errorf("popular tags = %v", global)
	}
	fleece, err := h.inventory.ItemTags(ctx, viewer.ID, "fleece")
	if err != nil {
		t.Fatalf("item tags: %v", err)
	}
	if !reflect.DeepEqual(fleece.Mine, []string{"warmwear"}) || len(fleece.Global) != 2 {
		t.Errorf("fleece tags for viewer = %+v, want mine [warmwear] and two global tags", fleece)
	}

	// A tag put in a metadata layer is not a tag. Only the tag store counts.
	if _, err := h.inventory.SetProfileLayer(ctx, core.ProfileItemLayer{
		ProfileID: owner.ID, ItemID: "tent",
		PublicMetadata: core.Metadata{core.CoreNamespace: map[string]interface{}{"tags": []interface{}{"warmwear"}}},
	}); err != nil {
		t.Fatalf("set layer: %v", err)
	}

	// Clearing a tag removes it from the owner's view.
	if _, err := h.inventory.SetItemTags(ctx, owner.ID, "tee", nil); err != nil {
		t.Fatalf("clear tags: %v", err)
	}
	detail, err = h.loadouts.Detail(ctx, created.Loadout.ID, owner.ID)
	if err != nil {
		t.Fatalf("detail: %v", err)
	}
	if got := detail.Filter.AvailableTags; len(got) != 1 || got[0].Tag != "coldwear" {
		t.Errorf("after clearing tee, available tags = %v", got)
	}
}

func TestSearchGear_ScopeDecidesWhoseGearAndWhoseTags(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	me := h.profile(t, "me")
	them := h.profile(t, "them")
	h.item(t, "cap", "headwear", 60, 0, false, nil)
	h.item(t, "rainhat", "headwear", 99, 0, false, nil)
	h.item(t, "beanie", "headwear", 40, 0, false, nil)
	h.item(t, "shell", "outerwear", 400, 0, false, nil)
	for _, tg := range []struct {
		profileID, itemID string
		tags              []string
	}{
		{me.ID, "cap", []string{"seattle", "rain"}},
		{me.ID, "beanie", []string{"coldwear"}},
		{them.ID, "rainhat", []string{"seattle"}},
		{them.ID, "cap", []string{"seattle"}},
		{them.ID, "shell", []string{"seattle"}},
	} {
		if _, err := h.inventory.SetItemTags(ctx, tg.profileID, tg.itemID, tg.tags); err != nil {
			t.Fatalf("tag: %v", err)
		}
	}
	ids := func(res core.GearSearch) []string {
		out := []string{}
		for _, r := range res.Results {
			out = append(out, r.Item.ID)
		}
		return out
	}

	everyone, err := h.inventory.SearchGear(ctx, core.GearQuery{Scope: core.ScopeEveryone, ProfileID: me.ID, Category: "headwear", Tags: []string{"#Seattle"}})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if got := ids(everyone); !reflect.DeepEqual(got, []string{"cap", "rainhat"}) {
		t.Errorf("everyone = %v, want [cap rainhat]", got)
	}
	if !everyone.Results[0].Mine || everyone.Results[1].Mine {
		t.Errorf("mine flags wrong: %+v", everyone.Results)
	}
	if want := []core.TagCount{{Tag: "seattle", Count: 2}}; !reflect.DeepEqual(everyone.Results[0].Tags[:1], want) {
		t.Errorf("cap global tags = %v", everyone.Results[0].Tags)
	}
	// Category facets ignore the category filter, so the shell is still offered.
	if len(everyone.CategoryFacets) != 2 {
		t.Errorf("category facets = %v, want headwear and outerwear", everyone.CategoryFacets)
	}

	// Under "mine", only my gear, and only my tags count.
	mine, err := h.inventory.SearchGear(ctx, core.GearQuery{Scope: core.ScopeMine, ProfileID: me.ID, Text: "#seattle"})
	if err != nil {
		t.Fatalf("search mine: %v", err)
	}
	if got := ids(mine); !reflect.DeepEqual(got, []string{"cap"}) {
		t.Errorf("mine = %v, want [cap]", got)
	}

	// Tags narrow: both must match.
	both, _ := h.inventory.SearchGear(ctx, core.GearQuery{Scope: core.ScopeEveryone, Tags: []string{"seattle", "rain"}})
	if got := ids(both); !reflect.DeepEqual(got, []string{"cap"}) {
		t.Errorf("seattle+rain = %v, want [cap]", got)
	}
}

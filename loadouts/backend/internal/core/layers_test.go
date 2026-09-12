package core

import "testing"

func testItem() Item {
	return Item{
		ID:       "tent_01",
		Name:     "Copper Spur UL2",
		Category: "shelter",
		ImageURL: "https://cdn.example.com/tent.jpg",
		BaseMetadata: Metadata{
			CoreNamespace: map[string]interface{}{
				KeyWeightG:    1360.0,
				KeyCostCents:  54995.0,
				KeyConsumable: false,
			},
		},
	}
}

func TestResolveLayers_OrderAndProvenance(t *testing.T) {
	item := testItem()
	in := LayerInput{
		Community: &CommunityItemLayer{
			CommunityID: "cmy_ul",
			ItemID:      item.ID,
			Metadata: Metadata{
				"ul_backpacking": map[string]interface{}{"ul_score": 7.8, "comfort": 8.1},
			},
		},
		Profile: &ProfileItemLayer{
			ProfileID: "prf_1",
			ItemID:    item.ID,
			PublicMetadata: Metadata{
				// The user weighed it themselves; their public layer wins over global.
				CoreNamespace:    map[string]interface{}{KeyWeightG: 1290.0},
				"ul_backpacking": map[string]interface{}{"comfort": 6.0},
			},
			PrivateMetadata: Metadata{
				"notes": map[string]interface{}{"text": "seam sealed"},
			},
			CustomImageURL: "https://my-photos.example.com/tent.jpg",
		},
	}
	ctx := LayerContext{ViewerProfileID: "prf_1", OwnerProfileID: "prf_1", CommunityID: "cmy_ul"}

	got := ResolveLayers(item, in, ctx)

	if got.WeightG() != 1290 {
		t.Errorf("user public layer should override global weight: got %v, want 1290", got.WeightG())
	}
	if got.CostCents() != 54995 {
		t.Errorf("untouched global key should survive: got %v, want 54995", got.CostCents())
	}

	ul, ok := got.Metadata["ul_backpacking"].(map[string]interface{})
	if !ok {
		t.Fatalf("community namespace missing from merged metadata: %+v", got.Metadata)
	}
	if ul["ul_score"] != 7.8 {
		t.Errorf("community value lost: got %v, want 7.8", ul["ul_score"])
	}
	if ul["comfort"] != 6.0 {
		t.Errorf("user layer should win over community: got %v, want 6.0", ul["comfort"])
	}

	if got.ImageURL != "https://my-photos.example.com/tent.jpg" {
		t.Errorf("custom image override failed: got %s", got.ImageURL)
	}

	wantProvenance := map[string]string{
		"core.weight_g":           LayerUserPublic,
		"core.cost_cents":         LayerGlobal,
		"ul_backpacking.ul_score": LayerCommunity,
		"ul_backpacking.comfort":  LayerUserPublic,
		"notes.text":              LayerUserPrivate,
	}
	for path, want := range wantProvenance {
		if got.Provenance[path] != want {
			t.Errorf("provenance[%s] = %q, want %q", path, got.Provenance[path], want)
		}
	}
}

func TestResolveLayers_PrivateLayerRedactedForOtherViewers(t *testing.T) {
	item := testItem()
	in := LayerInput{
		Profile: &ProfileItemLayer{
			ProfileID:       "prf_owner",
			ItemID:          item.ID,
			PublicMetadata:  Metadata{"spec": map[string]interface{}{"material": "20D nylon"}},
			PrivateMetadata: Metadata{"notes": map[string]interface{}{"text": "cracked pole"}},
		},
	}

	viewer := ResolveLayers(item, in, LayerContext{ViewerProfileID: "prf_other", OwnerProfileID: "prf_owner"})
	if _, leaked := viewer.Metadata["notes"]; leaked {
		t.Fatalf("private layer leaked to a non-owner viewer: %+v", viewer.Metadata)
	}
	if _, ok := viewer.Metadata["spec"]; !ok {
		t.Error("public user layer should still be visible to other viewers")
	}

	owner := ResolveLayers(item, in, LayerContext{ViewerProfileID: "prf_owner", OwnerProfileID: "prf_owner"})
	if _, ok := owner.Metadata["notes"]; !ok {
		t.Error("owner should see their own private layer")
	}
}

func TestResolveLayers_CommunityLayerOnlyAppliesInScope(t *testing.T) {
	item := testItem()
	in := LayerInput{
		Community: &CommunityItemLayer{
			CommunityID: "cmy_ul",
			ItemID:      item.ID,
			Metadata:    Metadata{"ul_backpacking": map[string]interface{}{"ul_score": 7.8}},
		},
	}

	// No community context: the community layer must not bleed into the global view.
	out := ResolveLayers(item, in, LayerContext{})
	if _, leaked := out.Metadata["ul_backpacking"]; leaked {
		t.Fatalf("community layer applied outside its scope: %+v", out.Metadata)
	}

	// Wrong community context is equally ignored.
	out = ResolveLayers(item, in, LayerContext{CommunityID: "cmy_other"})
	if _, leaked := out.Metadata["ul_backpacking"]; leaked {
		t.Fatalf("community layer applied for the wrong community: %+v", out.Metadata)
	}
}

func TestSlotDefinition_AcceptsAndCapacity(t *testing.T) {
	slot := SlotDefinition{ID: "shelter", AcceptedCategories: []string{"shelter"}}
	if !slot.Accepts("shelter") {
		t.Error("slot should accept its listed category")
	}
	if slot.Accepts("pack") {
		t.Error("slot should reject unlisted categories")
	}
	if slot.Capacity() != 1 {
		t.Errorf("unset MaxItems should mean a single item, got %d", slot.Capacity())
	}

	universal := SlotDefinition{ID: "main", AcceptedCategories: []string{"universal"}, MaxItems: -1}
	if !universal.Accepts("anything") {
		t.Error("universal slots should accept any category")
	}
	if universal.Capacity() != -1 {
		t.Errorf("unlimited capacity should be preserved, got %d", universal.Capacity())
	}
}

func TestLoadoutVisibility(t *testing.T) {
	private := Loadout{OwnerProfileID: "prf_owner", Visibility: VisibilityPrivate}
	if !private.IsVisibleTo("prf_owner") {
		t.Error("owner must always see their own loadout")
	}
	if private.IsVisibleTo("prf_other") || private.IsVisibleTo("") {
		t.Error("private loadouts must be hidden from everyone else")
	}

	public := Loadout{OwnerProfileID: "prf_owner", Visibility: VisibilityPublic}
	if !public.IsVisibleTo("") {
		t.Error("public loadouts should be visible anonymously")
	}
}

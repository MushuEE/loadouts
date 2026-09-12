package core

import (
	"errors"
	"strings"
	"testing"
)

// graph is a literal adjacency map: parent -> children.
type graph map[string][]string

func (g graph) children() ChildrenFunc {
	return func(id string) ([]string, error) { return g[id], nil }
}

// parents inverts the graph, which is what the store's reverse index does for real.
func (g graph) parents() ParentsFunc {
	inverted := map[string][]string{}
	for parent, kids := range g {
		for _, k := range kids {
			inverted[k] = append(inverted[k], parent)
		}
	}
	return func(id string) ([]string, error) { return inverted[id], nil }
}

func TestHeightBelow(t *testing.T) {
	//  trip -> meals -> snacks
	//       -> shelter
	//  A diamond: both days point at the same snack bag.
	g := graph{
		"trip":   {"day1", "day2"},
		"day1":   {"snacks"},
		"day2":   {"snacks"},
		"snacks": nil,
	}

	cases := map[string]int{
		"trip":    3, // trip -> day1 -> snacks
		"day1":    2,
		"snacks":  1,
		"unknown": 1, // a leaf we have never heard of is still a chain of one
	}
	for id, want := range cases {
		t.Run(id, func(t *testing.T) {
			got, err := HeightBelow(id, g.children())
			if err != nil {
				t.Fatalf("HeightBelow(%q): %v", id, err)
			}
			if got != want {
				t.Errorf("HeightBelow(%q) = %d, want %d", id, got, want)
			}
		})
	}
}

func TestDepthAbove(t *testing.T) {
	g := graph{
		"trip": {"day1"},
		"day1": {"snacks"},
	}
	cases := map[string]int{
		"trip":   1, // nothing references the trip
		"day1":   2,
		"snacks": 3,
	}
	for id, want := range cases {
		t.Run(id, func(t *testing.T) {
			got, err := DepthAbove(id, g.parents())
			if err != nil {
				t.Fatalf("DepthAbove(%q): %v", id, err)
			}
			if got != want {
				t.Errorf("DepthAbove(%q) = %d, want %d", id, got, want)
			}
		})
	}
}

// A diamond must not be re-explored, and must not be double counted into the height.
func TestHeightBelowHandlesDiamonds(t *testing.T) {
	g := graph{
		"root": {"a", "b"},
		"a":    {"shared"},
		"b":    {"shared"},
		"shared": {
			"leaf",
		},
	}
	got, err := HeightBelow("root", g.children())
	if err != nil {
		t.Fatalf("HeightBelow: %v", err)
	}
	if got != 4 { // root -> a -> shared -> leaf
		t.Errorf("HeightBelow(root) = %d, want 4", got)
	}
}

// A cycle already in the data must be reported, not spun on forever.
func TestLongestPathDetectsExistingCycle(t *testing.T) {
	g := graph{
		"a": {"b"},
		"b": {"c"},
		"c": {"a"},
	}
	if _, err := HeightBelow("a", g.children()); !errors.Is(err, ErrGraphCycle) {
		t.Fatalf("HeightBelow over a cycle returned %v, want ErrGraphCycle", err)
	}
}

func TestReaches(t *testing.T) {
	g := graph{
		"trip":  {"day1", "day2"},
		"day1":  {"snacks"},
		"other": {"day2"},
	}
	cases := []struct {
		from, target string
		want         bool
	}{
		{"trip", "snacks", true},
		{"trip", "day2", true},
		{"trip", "trip", true},  // a node reaches itself
		{"day1", "trip", false}, // references only point one way
		{"day2", "snacks", false},
		{"snacks", "trip", false},
	}
	for _, tc := range cases {
		t.Run(tc.from+"->"+tc.target, func(t *testing.T) {
			got, err := Reaches(tc.from, tc.target, g.children())
			if err != nil {
				t.Fatalf("Reaches: %v", err)
			}
			if got != tc.want {
				t.Errorf("Reaches(%q, %q) = %v, want %v", tc.from, tc.target, got, tc.want)
			}
		})
	}
}

func TestValidateAttachmentRefusesCycles(t *testing.T) {
	// trip -> day1 -> snacks
	g := graph{
		"trip": {"day1"},
		"day1": {"snacks"},
	}

	cases := []struct {
		name           string
		parent, child  string
		wantErr        bool
		wantMentioning string
	}{
		{name: "self reference", parent: "trip", child: "trip", wantErr: true, wantMentioning: "itself"},
		{name: "direct back edge", parent: "snacks", child: "trip", wantErr: true, wantMentioning: "loop"},
		{name: "indirect back edge", parent: "snacks", child: "day1", wantErr: true, wantMentioning: "loop"},
		{name: "a fresh sibling is fine", parent: "trip", child: "day2", wantErr: false},
		{name: "re-pointing deeper is fine", parent: "snacks", child: "wrapper", wantErr: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateAttachment(tc.parent, tc.child, g.children(), g.parents())
			if tc.wantErr {
				if err == nil {
					t.Fatalf("ValidateAttachment(%q, %q) succeeded, want an error", tc.parent, tc.child)
				}
				if tc.wantMentioning != "" && !contains(err.Error(), tc.wantMentioning) {
					t.Errorf("error %q does not mention %q", err, tc.wantMentioning)
				}
				return
			}
			if err != nil {
				t.Fatalf("ValidateAttachment(%q, %q): %v", tc.parent, tc.child, err)
			}
		})
	}
}

// The depth rule has to consider the chain above the parent, not just below the child.
// This is the case a one-directional check gets wrong.
func TestValidateAttachmentCountsBothDirections(t *testing.T) {
	// A chain of four: a -> b -> c -> d. MaxLoadoutDepth is 5.
	g := graph{
		"a": {"b"},
		"b": {"c"},
		"c": {"d"},
	}

	// Attaching a leaf to the bottom makes a chain of 5, which is exactly the limit.
	if err := ValidateAttachment("d", "leaf", g.children(), g.parents()); err != nil {
		t.Fatalf("attaching a leaf at depth 5 should be allowed: %v", err)
	}

	// Attaching a 2-deep subtree to the bottom would make 6.
	deep := graph{
		"a": {"b"}, "b": {"c"}, "c": {"d"},
		"sub": {"subchild"},
	}
	err := ValidateAttachment("d", "sub", deep.children(), deep.parents())
	if err == nil {
		t.Fatal("attaching a 2-deep subtree at depth 4 should exceed the limit")
	}
	if !contains(err.Error(), "deep") {
		t.Errorf("error %q should explain the depth limit", err)
	}

	// The same subtree attached to a root is fine: depth is a property of the chain,
	// not of either node alone.
	if err := ValidateAttachment("fresh", "sub", deep.children(), deep.parents()); err != nil {
		t.Fatalf("the same subtree under a root should be allowed: %v", err)
	}
}

// Attaching below something that is itself deeply referenced must fail even though the
// parent's own subtree is shallow.
func TestValidateAttachmentRespectsAncestors(t *testing.T) {
	g := graph{
		"l1": {"l2"},
		"l2": {"l3"},
		"l3": {"l4"},
		"l4": {"l5"},
	}
	// l5 is already at depth 5. Nothing may go beneath it.
	if err := ValidateAttachment("l5", "anything", g.children(), g.parents()); err == nil {
		t.Fatal("attaching beneath a node already at the depth limit should fail")
	}
	// But l1 has room.
	if err := ValidateAttachment("l1", "anything", g.children(), g.parents()); err != nil {
		t.Fatalf("attaching a second child to the root should be allowed: %v", err)
	}
}

func TestValidateSlots(t *testing.T) {
	cases := []struct {
		name    string
		slots   SlotList
		wantErr string
	}{
		{
			name:  "a plain item slot",
			slots: SlotList{{ID: "shelter", AcceptedCategories: []string{"shelter"}}},
		},
		{
			name:  "a sub-loadout slot",
			slots: SlotList{{ID: "food", AcceptedTemplateIDs: []string{"meals"}, MaxItems: -1}},
		},
		{
			name:    "a slot cannot be both",
			slots:   SlotList{{ID: "food", AcceptedCategories: []string{"food"}, AcceptedTemplateIDs: []string{"meals"}}},
			wantErr: "one or the other",
		},
		{
			name:    "duplicate ids",
			slots:   SlotList{{ID: "a"}, {ID: "a"}},
			wantErr: "duplicate",
		},
		{
			name:    "missing id",
			slots:   SlotList{{Name: "nameless"}},
			wantErr: "needs an id",
		},
		{
			name:    "unknown selection mode",
			slots:   SlotList{{ID: "a", MaxItems: -1, Selection: "average"}},
			wantErr: "selection mode",
		},
		{
			name:    "alternatives needs room for more than one",
			slots:   SlotList{{ID: "a", Selection: SelectionAlternatives}},
			wantErr: "nothing to choose between",
		},
		{
			name:  "alternatives with capacity is fine",
			slots: SlotList{{ID: "a", MaxItems: 3, Selection: SelectionAlternatives}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateSlots(tc.slots)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected an error mentioning %q", tc.wantErr)
			}
			if !contains(err.Error(), tc.wantErr) {
				t.Errorf("error %q does not mention %q", err, tc.wantErr)
			}
		})
	}
}

func TestSlotAcceptsTemplate(t *testing.T) {
	item := SlotDefinition{ID: "shelter", AcceptedCategories: []string{"shelter"}}
	specific := SlotDefinition{ID: "food", AcceptedTemplateIDs: []string{"meals", "resupply"}}
	any := SlotDefinition{ID: "extra", IsSubLoadout: true}

	if item.AcceptsTemplate("meals") {
		t.Error("an item slot must not accept a template")
	}
	if item.HoldsSubLoadouts() {
		t.Error("an item slot must not report as a sub-loadout slot")
	}
	if !specific.AcceptsTemplate("resupply") {
		t.Error("a listed template should be accepted")
	}
	if specific.AcceptsTemplate("platform-freeform") {
		t.Error("an unlisted template should be refused")
	}
	if !any.AcceptsTemplate("anything") {
		t.Error("an empty template list means any template")
	}
	// A sub-loadout slot rejects items, which is the mirror of the rule above.
	if specific.Accepts("food") {
		t.Error("a sub-loadout slot must not accept an item category")
	}
}

func TestSelectionModeNormalize(t *testing.T) {
	if SelectionMode("").Normalize() != SelectionSum {
		t.Error("the zero value should normalize to sum")
	}
	if SelectionAlternatives.Normalize() != SelectionAlternatives {
		t.Error("an explicit mode should be preserved")
	}
	if SelectionMode("nonsense").IsValid() {
		t.Error("an unknown mode should not validate")
	}
}

func TestLoadoutStatsAdd(t *testing.T) {
	parent := LoadoutStats{TotalWeightG: 100, BaseWeightG: 100, ItemCount: 1}
	child := LoadoutStats{TotalWeightG: 700, ConsumableWeightG: 700, TotalCostCents: 1200, ItemCount: 5}

	parent.Add(child, 3) // three days of the same meals
	if parent.TotalWeightG != 2200 {
		t.Errorf("TotalWeightG = %v, want 2200", parent.TotalWeightG)
	}
	if parent.ConsumableWeightG != 2100 {
		t.Errorf("ConsumableWeightG = %v, want 2100", parent.ConsumableWeightG)
	}
	if parent.BaseWeightG != 100 {
		t.Errorf("BaseWeightG = %v, want 100; consumables must not leak into base", parent.BaseWeightG)
	}
	if parent.ItemCount != 16 {
		t.Errorf("ItemCount = %d, want 16", parent.ItemCount)
	}

	// A zero or negative multiplier is treated as one rather than erasing the child.
	zeroed := LoadoutStats{}
	zeroed.Add(child, 0)
	if zeroed.TotalWeightG != 700 {
		t.Errorf("Add with times=0 gave %v, want the child counted once", zeroed.TotalWeightG)
	}
}

func contains(haystack, needle string) bool { return strings.Contains(haystack, needle) }

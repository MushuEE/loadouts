package core

import (
	"strings"
	"testing"
)

func entry(id, slot, item string, qty, position int) LoadoutEntry {
	return LoadoutEntry{ID: id, SlotID: slot, ItemID: item, Quantity: qty, Position: position}
}

func TestFingerprintIsStableAcrossEntryOrder(t *testing.T) {
	l := Loadout{Name: "UL Budget Kit"}
	a := []LoadoutEntry{
		entry("e1", "shelter", "tent", 1, 0),
		entry("e2", "sleep", "quilt", 1, 1),
		entry("e3", "cook", "pot", 1, 2),
	}
	// The store makes no ordering promise across backends, so the digest must not care.
	b := []LoadoutEntry{a[2], a[0], a[1]}

	if FingerprintLoadout(l, a) != FingerprintLoadout(l, b) {
		t.Error("fingerprint changed when the same entries arrived in a different order")
	}
}

func TestFingerprintIgnoresCosmeticChanges(t *testing.T) {
	entries := []LoadoutEntry{entry("e1", "shelter", "tent", 1, 0)}
	base := Loadout{Name: "Kit", Description: "before", CoverImageURL: "a.png"}

	cosmetic := base
	cosmetic.Description = "after, with a much longer and more detailed writeup"
	cosmetic.CoverImageURL = "b.png"
	if FingerprintLoadout(base, entries) != FingerprintLoadout(cosmetic, entries) {
		t.Error("editing the description or cover should not invalidate an endorsement")
	}

	// Dragging a card to a different grid cell is not a change of substance. An
	// endorsement that cries wolf on every tidy-up will be ignored.
	moved := []LoadoutEntry{entry("e1", "shelter", "tent", 1, 7)}
	if FingerprintLoadout(base, entries) != FingerprintLoadout(base, moved) {
		t.Error("repositioning an entry should not invalidate an endorsement")
	}

	// Per-entry notes are the owner talking to themselves.
	noted := []LoadoutEntry{entry("e1", "shelter", "tent", 1, 0)}
	noted[0].Note = "remember the footprint"
	if FingerprintLoadout(base, entries) != FingerprintLoadout(base, noted) {
		t.Error("an entry note should not invalidate an endorsement")
	}
}

func TestFingerprintCatchesSubstantiveChanges(t *testing.T) {
	base := Loadout{Name: "UL Budget Kit"}
	entries := []LoadoutEntry{
		entry("e1", "shelter", "tent", 1, 0),
		entry("e2", "sleep", "quilt", 1, 1),
	}
	original := FingerprintLoadout(base, entries)

	cases := []struct {
		name    string
		loadout Loadout
		entries []LoadoutEntry
	}{
		{
			// The headline hijack: same gear list, new marketing.
			name:    "renamed",
			loadout: Loadout{Name: "Sponsored Kit"},
			entries: entries,
		},
		{
			name:    "item swapped",
			loadout: base,
			entries: []LoadoutEntry{entry("e1", "shelter", "sponsored-tent", 1, 0), entries[1]},
		},
		{
			name:    "quantity changed",
			loadout: base,
			entries: []LoadoutEntry{entry("e1", "shelter", "tent", 3, 0), entries[1]},
		},
		{
			name:    "entry removed",
			loadout: base,
			entries: entries[:1],
		},
		{
			name:    "entry added",
			loadout: base,
			entries: append(append([]LoadoutEntry{}, entries...), entry("e3", "cook", "pot", 1, 2)),
		},
		{
			name:    "moved to a different slot",
			loadout: base,
			entries: []LoadoutEntry{entry("e1", "spare", "tent", 1, 0), entries[1]},
		},
		{
			// A sub-loadout swap: the trip's own rows barely move, but what it carries
			// is entirely different.
			name:    "sub-loadout swapped",
			loadout: base,
			entries: []LoadoutEntry{
				{ID: "e1", SlotID: "food", ChildLoadoutID: "ldt_meals_day1", Quantity: 1, Selected: true},
			},
		},
		{
			name:    "a different alternative is selected",
			loadout: base,
			entries: []LoadoutEntry{
				{ID: "e1", SlotID: "food", ChildLoadoutID: "ldt_meals_day1", Quantity: 1, Selected: false},
			},
		},
		{
			name:    "renested under a different parent",
			loadout: base,
			entries: []LoadoutEntry{{ID: "e1", SlotID: "shelter", ItemID: "tent", Quantity: 1, ParentEntryID: "e2"}, entries[1]},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if FingerprintLoadout(tc.loadout, tc.entries) == original {
				t.Errorf("%s should have changed the fingerprint", tc.name)
			}
		})
	}
}

// The name is length-prefixed precisely so it cannot be confused with entry data.
// Selecting a different alternative in a swipeable slot changes what the loadout
// actually weighs, so it must not be invisible to an endorsement.
func TestFingerprintDistinguishesSelectionState(t *testing.T) {
	l := Loadout{Name: "Trip"}
	selected := []LoadoutEntry{{ID: "e1", SlotID: "food", ChildLoadoutID: "ldt_day1", Quantity: 1, Selected: true}}
	deselected := []LoadoutEntry{{ID: "e1", SlotID: "food", ChildLoadoutID: "ldt_day1", Quantity: 1, Selected: false}}
	if FingerprintLoadout(l, selected) == FingerprintLoadout(l, deselected) {
		t.Error("selection state must be part of the digest")
	}

	// And an item entry must never collide with a sub-loadout entry that happens to
	// share an ID and slot.
	item := []LoadoutEntry{{ID: "e1", SlotID: "food", ItemID: "ldt_day1", Quantity: 1, Selected: true}}
	if FingerprintLoadout(l, selected) == FingerprintLoadout(l, item) {
		t.Error("a sub-loadout reference collided with an item of the same ID")
	}
}

func TestFingerprintNameCannotImpersonateAnEntry(t *testing.T) {
	a := FingerprintLoadout(Loadout{Name: "Kit"}, []LoadoutEntry{entry("e1", "s", "tent", 1, 0)})
	b := FingerprintLoadout(Loadout{Name: "Kit\ne1|s||tent|1"}, nil)
	if a == b {
		t.Error("a crafted name collided with an entry line")
	}
}

func TestFingerprintIsAlgorithmTagged(t *testing.T) {
	fp := FingerprintLoadout(Loadout{Name: "Kit"}, nil)
	if !strings.HasPrefix(fp, "fp2:") {
		t.Errorf("fingerprint %q should carry its algorithm tag so it can be changed later", fp)
	}
	// An empty loadout still fingerprints; "no entries" is a real, comparable state.
	if len(fp) <= len("fp2:") {
		t.Error("an empty loadout should still produce a digest")
	}
}

func TestFavoriteScopeValidity(t *testing.T) {
	for _, s := range []FavoriteScope{FavoriteProfile, FavoriteCommunity} {
		if !s.IsValid() {
			t.Errorf("%q should be a valid scope", s)
		}
	}
	for _, s := range []FavoriteScope{"", "platform", "Profile", "user"} {
		if s.IsValid() {
			t.Errorf("%q should not be a valid scope", s)
		}
	}
}

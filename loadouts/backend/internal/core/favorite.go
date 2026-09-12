package core

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strconv"
	"strings"
	"time"
)

// FavoriteScope is who holds a favorite: one profile, or one community.
type FavoriteScope string

const (
	FavoriteProfile   FavoriteScope = "profile"   // A private bookmark
	FavoriteCommunity FavoriteScope = "community" // A public endorsement
)

func (s FavoriteScope) IsValid() bool {
	return s == FavoriteProfile || s == FavoriteCommunity
}

// Favorite is a durable pointer at a loadout that the holder does not own.
//
// It is deliberately not ownership and not a copy. A community endorsing a member's
// canonical meal kit gets a visible, reversible signal — "this is the good one" — while
// the kit stays under its author's sole control, so no member's trip can change weight
// because a moderator edited something.
//
// Fingerprint is the one piece of cleverness: the substance of the loadout at the moment
// this favorite was last confirmed. Comparing it to the loadout's current fingerprint
// turns a silent swap of the endorsed content into a visible "edited since endorsed".
type Favorite struct {
	ID             string        `json:"id" db:"id"`
	ScopeType      FavoriteScope `json:"scope_type" db:"scope_type"`
	ScopeID        string        `json:"scope_id" db:"scope_id"`
	LoadoutID      string        `json:"loadout_id" db:"loadout_id"`
	Note           string        `json:"note" db:"note"`
	Fingerprint    string        `json:"fingerprint" db:"fingerprint"`
	ActorProfileID string        `json:"actor_profile_id" db:"actor_profile_id"`
	CreatedAt      time.Time     `json:"created_at" db:"created_at"`
	ConfirmedAt    time.Time     `json:"confirmed_at" db:"confirmed_at"`
}

// fingerprintAlgorithm prefixes every digest so the hashed inputs can change later
// without anyone having to guess how an old value was produced.
//
// fp2 added ChildLoadoutID and Selected, once sub-loadouts existed to reference. Bumping
// the tag means every fp1 endorsement reads as stale exactly once, which is the correct
// answer: those digests genuinely could not see a whole category of change.
const fingerprintAlgorithm = "fp2"

// FingerprintLoadout digests the *substance* of a loadout: its name and the set of things
// in it. Two loadouts with the same fingerprint carry the same gear under the same name.
//
// What is excluded is as deliberate as what is included:
//
//   - Position is out. Dragging cards around in the UI is cosmetic, and an endorsement
//     that cries wolf every time someone tidies their list will be ignored.
//   - Description and cover image are out, for the same reason.
//   - Name is in. "UL Budget Kit" quietly becoming "Sponsored Kit" is precisely the
//     substitution this exists to catch.
//   - ChildLoadoutID and Selected are in. Swapping which meal kit hangs off the food
//     slot, or which alternative is the live one, changes what was endorsed as surely as
//     swapping an item does.
//
// The digest remains **local**: it covers this loadout's own rows, and does not follow
// ChildLoadoutID into the referenced loadout's contents. Whoever edits "Meals Day 1"
// changes that loadout's own fingerprint, so an endorsement *of the meal kit* goes stale
// correctly — but an endorsement of a trip that references it does not. Chasing the whole
// subtree would make every parent's freshness depend on strangers' edits, which is a
// different and much noisier signal; it wants its own decision rather than being smuggled
// in here.
func FingerprintLoadout(l Loadout, entries []LoadoutEntry) string {
	lines := make([]string, 0, len(entries))
	for _, e := range entries {
		// Entry ID leads so the sort is stable regardless of how the store returned them,
		// and so two entries that are otherwise identical stay distinct.
		lines = append(lines, strings.Join([]string{
			e.ID,
			e.SlotID,
			e.ParentEntryID,
			e.ItemID,
			e.ChildLoadoutID,
			strconv.Itoa(e.Quantity),
			strconv.FormatBool(e.Selected),
		}, "|"))
	}
	sort.Strings(lines)

	h := sha256.New()
	// The name is length-prefixed so a name containing the separator cannot impersonate
	// an entry line.
	h.Write([]byte(strconv.Itoa(len(l.Name))))
	h.Write([]byte{'\n'})
	h.Write([]byte(l.Name))
	h.Write([]byte{'\n'})
	for _, line := range lines {
		h.Write([]byte(line))
		h.Write([]byte{'\n'})
	}
	return fingerprintAlgorithm + ":" + hex.EncodeToString(h.Sum(nil))
}

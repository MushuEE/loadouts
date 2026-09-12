package service

import (
	"context"
	"errors"
	"testing"

	"github.com/gmccloskey/loadouts/backend/internal/core"
)

// favoriteFixture builds the recurring cast: a community with an admin and a plain
// member, plus a published loadout owned by a third party that nobody else controls.
type favoriteFixture struct {
	admin     core.Profile
	member    core.Profile
	author    core.Profile
	community core.Community
	loadout   core.LoadoutDetail
}

func newFavoriteFixture(t *testing.T, h *harness) favoriteFixture {
	t.Helper()
	ctx := context.Background()

	admin := h.profile(t, "admin")
	member := h.profile(t, "member")
	author := h.profile(t, "author")
	h.item(t, "stove", "cook", 80, 4500, false, nil)

	community, err := h.community.Create(ctx, admin.ID, core.Community{Name: "UL Backpacking"})
	if err != nil {
		t.Fatalf("create community: %v", err)
	}
	if _, err := h.community.Join(ctx, community.ID, member.ID); err != nil {
		t.Fatalf("join: %v", err)
	}

	loadout, err := h.loadouts.Create(ctx, author.ID, CreateLoadoutRequest{
		Name:       "UL Budget Meal Kit",
		Visibility: core.VisibilityPublic,
		Entries: []core.LoadoutEntry{
			{ID: "e1", SlotID: "gear", ItemID: "stove", Quantity: 1},
		},
	})
	if err != nil {
		t.Fatalf("create loadout: %v", err)
	}
	return favoriteFixture{admin: admin, member: member, author: author, community: community, loadout: loadout}
}

func TestFavorite_CommunityEndorsementNeedsAnAdmin(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	f := newFavoriteFixture(t, h)

	// A plain member cannot speak for the community.
	if _, err := h.favorites.Favorite(ctx, f.member.ID, core.FavoriteCommunity, f.community.ID, f.loadout.Loadout.ID, ""); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("member endorsing for the community should be forbidden, got %v", err)
	}
	// Nor can the loadout's own author endorse it on the community's behalf.
	if _, err := h.favorites.Favorite(ctx, f.author.ID, core.FavoriteCommunity, f.community.ID, f.loadout.Loadout.ID, ""); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("author self-endorsing for the community should be forbidden, got %v", err)
	}

	view, err := h.favorites.Favorite(ctx, f.admin.ID, core.FavoriteCommunity, f.community.ID, f.loadout.Loadout.ID, "the best budget kit")
	if err != nil {
		t.Fatalf("admin endorsement: %v", err)
	}
	if !view.Available || view.Stale {
		t.Errorf("a fresh endorsement should be available and not stale, got available=%v stale=%v", view.Available, view.Stale)
	}
	if view.Note != "the best budget kit" {
		t.Errorf("note = %q", view.Note)
	}
	if view.Loadout == nil || view.Loadout.Loadout.ID != f.loadout.Loadout.ID {
		t.Error("the view should carry a summary of the endorsed loadout")
	}
	// Endorsement does not require, or grant, any power over the loadout.
	name := "Hijacked"
	if _, err := h.loadouts.Update(ctx, f.admin.ID, f.loadout.Loadout.ID, UpdateLoadoutRequest{Name: &name}); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("endorsing must not confer edit rights, got %v", err)
	}
}

// The headline scenario the fingerprint exists for.
func TestFavorite_GoesStaleWhenTheEndorsedContentChanges(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	f := newFavoriteFixture(t, h)
	h.item(t, "sponsored-stove", "cook", 400, 19900, false, nil)

	if _, err := h.favorites.Favorite(ctx, f.admin.ID, core.FavoriteCommunity, f.community.ID, f.loadout.Loadout.ID, ""); err != nil {
		t.Fatalf("endorse: %v", err)
	}

	// A cosmetic edit by the owner must not disturb the endorsement.
	description := "now with more detail about why this kit works"
	if _, err := h.loadouts.Update(ctx, f.author.ID, f.loadout.Loadout.ID, UpdateLoadoutRequest{Description: &description}); err != nil {
		t.Fatalf("cosmetic update: %v", err)
	}
	views, err := h.favorites.ListForScope(ctx, f.admin.ID, core.FavoriteCommunity, f.community.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(views) != 1 || views[0].Stale {
		t.Fatalf("a description edit should not make an endorsement stale, got %+v", views)
	}

	// Now the actual hijack: swap the gear for sponsored kit and rename it.
	if _, err := h.loadouts.ReplaceEntries(ctx, f.author.ID, f.loadout.Loadout.ID, []core.LoadoutEntry{
		{ID: "e1", SlotID: "gear", ItemID: "sponsored-stove", Quantity: 1},
	}); err != nil {
		t.Fatalf("replace entries: %v", err)
	}
	views, err = h.favorites.ListForScope(ctx, f.admin.ID, core.FavoriteCommunity, f.community.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(views) != 1 || !views[0].Stale {
		t.Fatalf("swapping the gear should make the endorsement stale, got %+v", views)
	}

	// Re-confirming clears the flag and keeps the original endorsement date.
	before := views[0].CreatedAt
	reconfirmed, err := h.favorites.Favorite(ctx, f.admin.ID, core.FavoriteCommunity, f.community.ID, f.loadout.Loadout.ID, "re-checked")
	if err != nil {
		t.Fatalf("re-confirm: %v", err)
	}
	if reconfirmed.Stale {
		t.Error("re-confirming should clear staleness")
	}
	if !reconfirmed.CreatedAt.Equal(before) {
		t.Errorf("re-confirming moved CreatedAt from %v to %v; the endorsement dates from when it was first given", before, reconfirmed.CreatedAt)
	}
	if !reconfirmed.ConfirmedAt.After(before) && !reconfirmed.ConfirmedAt.Equal(before) {
		t.Error("ConfirmedAt should advance on re-confirmation")
	}
}

func TestFavorite_IsIdempotent(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	f := newFavoriteFixture(t, h)

	for i := 0; i < 3; i++ {
		if _, err := h.favorites.Favorite(ctx, f.admin.ID, core.FavoriteCommunity, f.community.ID, f.loadout.Loadout.ID, ""); err != nil {
			t.Fatalf("endorse %d: %v", i, err)
		}
	}
	views, err := h.favorites.ListForScope(ctx, f.admin.ID, core.FavoriteCommunity, f.community.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(views) != 1 {
		t.Fatalf("favoriting three times produced %d rows, want 1", len(views))
	}
}

func TestFavorite_CommunityCannotEndorseAPrivateLoadout(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	f := newFavoriteFixture(t, h)

	// The admin's own private draft: visible to them, but useless as a public endorsement
	// and a disclosure that it exists.
	draft, err := h.loadouts.Create(ctx, f.admin.ID, CreateLoadoutRequest{Name: "Secret", Visibility: core.VisibilityPrivate})
	if err != nil {
		t.Fatalf("create draft: %v", err)
	}
	if _, err := h.favorites.Favorite(ctx, f.admin.ID, core.FavoriteCommunity, f.community.ID, draft.Loadout.ID, ""); !errors.Is(err, core.ErrInvalid) {
		t.Errorf("community endorsement of a private loadout should be rejected, got %v", err)
	}
	// But bookmarking your own draft privately is exactly what bookmarks are for.
	if _, err := h.favorites.Favorite(ctx, f.admin.ID, core.FavoriteProfile, f.admin.ID, draft.Loadout.ID, ""); err != nil {
		t.Errorf("bookmarking your own private draft should be allowed: %v", err)
	}
	// And you cannot bookmark something you cannot see.
	if _, err := h.favorites.Favorite(ctx, f.member.ID, core.FavoriteProfile, f.member.ID, draft.Loadout.ID, ""); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("bookmarking someone else's private draft should be forbidden, got %v", err)
	}
}

func TestFavorite_BookmarksAreScopedToTheirOwner(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	f := newFavoriteFixture(t, h)

	// You cannot file a bookmark into someone else's shelf.
	if _, err := h.favorites.Favorite(ctx, f.member.ID, core.FavoriteProfile, f.admin.ID, f.loadout.Loadout.ID, ""); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("bookmarking to another profile should be forbidden, got %v", err)
	}
	if _, err := h.favorites.Favorite(ctx, f.member.ID, core.FavoriteProfile, f.member.ID, f.loadout.Loadout.ID, ""); err != nil {
		t.Fatalf("own bookmark: %v", err)
	}
	// Nor read someone else's shelf.
	if _, err := h.favorites.ListForScope(ctx, f.admin.ID, core.FavoriteProfile, f.member.ID); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("reading another profile's bookmarks should be forbidden, got %v", err)
	}
	// A community's endorsements, by contrast, are public: that is the point of them.
	if _, err := h.favorites.ListForScope(ctx, "", core.FavoriteCommunity, f.community.ID); err != nil {
		t.Errorf("community endorsements should be publicly listable: %v", err)
	}
}

// "Who vouches for this?" must not become "who privately saved this?".
func TestFavorite_LoadoutPageShowsOnlyCommunityEndorsements(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	f := newFavoriteFixture(t, h)

	if _, err := h.favorites.Favorite(ctx, f.member.ID, core.FavoriteProfile, f.member.ID, f.loadout.Loadout.ID, ""); err != nil {
		t.Fatalf("bookmark: %v", err)
	}
	if _, err := h.favorites.Favorite(ctx, f.admin.ID, core.FavoriteCommunity, f.community.ID, f.loadout.Loadout.ID, ""); err != nil {
		t.Fatalf("endorse: %v", err)
	}

	views, err := h.favorites.ListForLoadout(ctx, f.loadout.Loadout.ID, f.author.ID)
	if err != nil {
		t.Fatalf("list for loadout: %v", err)
	}
	if len(views) != 1 {
		t.Fatalf("got %d public endorsements, want 1 (private bookmarks must not leak)", len(views))
	}
	if views[0].ScopeType != core.FavoriteCommunity {
		t.Errorf("scope = %q, want community", views[0].ScopeType)
	}
}

// An endorsement outliving its target is the deliberate behaviour, not a bug.
func TestFavorite_SurvivesTheLoadoutGoingAway(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	f := newFavoriteFixture(t, h)

	second, err := h.loadouts.Create(ctx, f.author.ID, CreateLoadoutRequest{Name: "Second Kit", Visibility: core.VisibilityPublic})
	if err != nil {
		t.Fatalf("create second: %v", err)
	}
	for _, id := range []string{f.loadout.Loadout.ID, second.Loadout.ID} {
		if _, err := h.favorites.Favorite(ctx, f.admin.ID, core.FavoriteCommunity, f.community.ID, id, ""); err != nil {
			t.Fatalf("endorse %s: %v", id, err)
		}
	}

	// The owner pulls one private and deletes the other.
	private := core.VisibilityPrivate
	if _, err := h.loadouts.Update(ctx, f.author.ID, f.loadout.Loadout.ID, UpdateLoadoutRequest{Visibility: &private}); err != nil {
		t.Fatalf("make private: %v", err)
	}
	if err := h.loadouts.Delete(ctx, f.author.ID, second.Loadout.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	views, err := h.favorites.ListForScope(ctx, f.admin.ID, core.FavoriteCommunity, f.community.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(views) != 2 {
		t.Fatalf("got %d endorsements, want 2 kept as broken references rather than silently dropped", len(views))
	}
	for _, v := range views {
		if v.Available {
			t.Errorf("endorsement of %s should be unavailable", v.LoadoutID)
		}
		if v.Loadout != nil {
			t.Errorf("an unavailable endorsement must not leak a summary of %s", v.LoadoutID)
		}
	}
}

func TestFavorite_Unfavorite(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	f := newFavoriteFixture(t, h)

	if _, err := h.favorites.Favorite(ctx, f.admin.ID, core.FavoriteCommunity, f.community.ID, f.loadout.Loadout.ID, ""); err != nil {
		t.Fatalf("endorse: %v", err)
	}
	// A member cannot withdraw the community's endorsement.
	if err := h.favorites.Unfavorite(ctx, f.member.ID, core.FavoriteCommunity, f.community.ID, f.loadout.Loadout.ID); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("member withdrawing should be forbidden, got %v", err)
	}
	if err := h.favorites.Unfavorite(ctx, f.admin.ID, core.FavoriteCommunity, f.community.ID, f.loadout.Loadout.ID); err != nil {
		t.Fatalf("admin withdrawing: %v", err)
	}
	views, err := h.favorites.ListForScope(ctx, f.admin.ID, core.FavoriteCommunity, f.community.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(views) != 0 {
		t.Fatalf("got %d endorsements after withdrawal, want 0", len(views))
	}
	// Withdrawing again is not an error: the caller wanted it gone, and it is gone.
	if err := h.favorites.Unfavorite(ctx, f.admin.ID, core.FavoriteCommunity, f.community.ID, f.loadout.Loadout.ID); err != nil {
		t.Errorf("withdrawing a missing endorsement should be a no-op, got %v", err)
	}
}

func TestFavorite_RejectsBadInput(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	f := newFavoriteFixture(t, h)

	if _, err := h.favorites.Favorite(ctx, f.admin.ID, "platform", "x", f.loadout.Loadout.ID, ""); !errors.Is(err, core.ErrInvalid) {
		t.Errorf("an unknown scope should be rejected, got %v", err)
	}
	if _, err := h.favorites.Favorite(ctx, "", core.FavoriteProfile, f.member.ID, f.loadout.Loadout.ID, ""); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("anonymous favoriting should be forbidden, got %v", err)
	}
	if _, err := h.favorites.Favorite(ctx, f.admin.ID, core.FavoriteCommunity, "cmy_missing", f.loadout.Loadout.ID, ""); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("a missing community should be not-found, got %v", err)
	}
	if _, err := h.favorites.Favorite(ctx, f.admin.ID, core.FavoriteProfile, f.admin.ID, "ldt_missing", ""); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("a missing loadout should be not-found, got %v", err)
	}
	long := make([]byte, maxNoteLength+1)
	for i := range long {
		long[i] = 'x'
	}
	if _, err := h.favorites.Favorite(ctx, f.admin.ID, core.FavoriteProfile, f.admin.ID, f.loadout.Loadout.ID, string(long)); !errors.Is(err, core.ErrInvalid) {
		t.Errorf("an oversized note should be rejected, got %v", err)
	}
}

package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gmccloskey/loadouts/backend/internal/core"
	"github.com/gmccloskey/loadouts/backend/internal/db"
)

// FavoriteService handles endorsement: a community saying "this is the good meal kit", or
// a profile bookmarking one for later.
//
// The deliberate shape here is that a favorite confers no authority. Nothing in the write
// path of any other service consults it, so it never has to be checked before a mutation
// and cannot grow into a permission system. It points, and that is all.
type FavoriteService struct {
	store     db.Store
	loadouts  *LoadoutService
	community *CommunityService
}

func NewFavoriteService(store db.Store, loadouts *LoadoutService, community *CommunityService) *FavoriteService {
	return &FavoriteService{store: store, loadouts: loadouts, community: community}
}

// FavoriteView is a favorite plus the two things only the service can work out: whether
// the target is still there for this viewer, and whether it changed since it was endorsed.
type FavoriteView struct {
	core.Favorite
	// Available is false when the loadout was deleted, or when its owner has since made
	// it invisible to this viewer. The favorite is still returned, because a community
	// that endorsed six kits and now sees five deserves to know why.
	Available bool `json:"available"`
	// Stale means the loadout's substance changed since this endorsement was confirmed.
	// It is the signal that a mod should look again and re-confirm.
	Stale bool `json:"stale"`
	// ScopeName is the community name or profile handle behind ScopeID. Resolved here
	// because "endorsed by UL Backpacking" is the entire value of an endorsement, and
	// making the client fetch a name per row to say it would be absurd.
	ScopeName string               `json:"scope_name"`
	Loadout   *core.LoadoutSummary `json:"loadout,omitempty"`
}

// maxNoteLength keeps an endorsement note to a sentence or two. It is a label, not a post.
const maxNoteLength = 500

// Favorite endorses a loadout, or re-confirms an existing endorsement. It is idempotent:
// favoriting twice updates the row rather than creating a second one, because
// re-confirming a stale endorsement is the same gesture as making it in the first place.
func (s *FavoriteService) Favorite(ctx context.Context, actorProfileID string, scopeType core.FavoriteScope, scopeID, loadoutID, note string) (FavoriteView, error) {
	if !scopeType.IsValid() {
		return FavoriteView{}, fmt.Errorf("%w: scope_type must be profile or community", core.ErrInvalid)
	}
	if len(note) > maxNoteLength {
		return FavoriteView{}, fmt.Errorf("%w: note is limited to %d characters", core.ErrInvalid, maxNoteLength)
	}
	if err := s.requireScopeAuthority(ctx, actorProfileID, scopeType, scopeID); err != nil {
		return FavoriteView{}, err
	}

	loadout, err := s.store.GetLoadout(ctx, loadoutID)
	if err != nil {
		return FavoriteView{}, fmt.Errorf("%w: loadout %s", core.ErrNotFound, loadoutID)
	}
	// You cannot endorse what you cannot see. This also stops a favorite being used to
	// probe for the existence of other people's private loadouts.
	if !loadout.IsVisibleTo(actorProfileID) {
		return FavoriteView{}, fmt.Errorf("%w: loadout %s is private", core.ErrForbidden, loadoutID)
	}
	// A community endorsement of a private loadout is a dead link for every member, and
	// it advertises that the private loadout exists. A personal bookmark of your own
	// draft is fine, so this rule is specific to community scope.
	if scopeType == core.FavoriteCommunity && loadout.Visibility == core.VisibilityPrivate {
		return FavoriteView{}, fmt.Errorf("%w: a community cannot endorse a private loadout", core.ErrInvalid)
	}

	entries, err := s.store.ListLoadoutEntries(ctx, loadoutID)
	if err != nil {
		return FavoriteView{}, err
	}

	now := time.Now().UTC()
	favorite := core.Favorite{
		ID:             core.NewID("fav"),
		ScopeType:      scopeType,
		ScopeID:        scopeID,
		LoadoutID:      loadoutID,
		Note:           strings.TrimSpace(note),
		Fingerprint:    core.FingerprintLoadout(loadout, entries),
		ActorProfileID: actorProfileID,
		CreatedAt:      now,
		ConfirmedAt:    now,
	}
	// The store preserves the original ID and CreatedAt on a re-confirmation, so an
	// endorsement is dated from when it was first given.
	if err := s.store.UpsertFavorite(ctx, favorite); err != nil {
		return FavoriteView{}, err
	}

	stored, err := s.store.GetFavorite(ctx, scopeType, scopeID, loadoutID)
	if err != nil || stored == nil {
		return FavoriteView{}, fmt.Errorf("favorite was written but could not be read back: %w", err)
	}
	return s.view(ctx, *stored, loadout, actorProfileID), nil
}

// Unfavorite withdraws an endorsement. Withdrawing one that is not there is not an error:
// the caller wanted it gone, and it is gone.
func (s *FavoriteService) Unfavorite(ctx context.Context, actorProfileID string, scopeType core.FavoriteScope, scopeID, loadoutID string) error {
	if !scopeType.IsValid() {
		return fmt.Errorf("%w: scope_type must be profile or community", core.ErrInvalid)
	}
	if err := s.requireScopeAuthority(ctx, actorProfileID, scopeType, scopeID); err != nil {
		return err
	}
	return s.store.DeleteFavorite(ctx, scopeType, scopeID, loadoutID)
}

// ListForScope is a shelf: everything this profile bookmarked, or everything this
// community endorses.
//
// Unavailable entries are kept in the list rather than filtered out. A silently shrinking
// endorsement list is the same mistake as a cascading delete — it destroys the evidence
// that something went away.
func (s *FavoriteService) ListForScope(ctx context.Context, viewerProfileID string, scopeType core.FavoriteScope, scopeID string) ([]FavoriteView, error) {
	if !scopeType.IsValid() {
		return nil, fmt.Errorf("%w: scope_type must be profile or community", core.ErrInvalid)
	}
	// A profile's bookmarks are private to that profile; a community's endorsements are
	// public, which is the entire point of making them.
	if scopeType == core.FavoriteProfile && scopeID != viewerProfileID {
		return nil, fmt.Errorf("%w: bookmarks are private to their profile", core.ErrForbidden)
	}

	favorites, err := s.store.ListFavoritesForScope(ctx, scopeType, scopeID)
	if err != nil {
		return nil, err
	}
	views := make([]FavoriteView, 0, len(favorites))
	for _, f := range favorites {
		loadout, err := s.store.GetLoadout(ctx, f.LoadoutID)
		if err != nil {
			// Deleted: Available stays false, but the endorsement still names its scope.
			views = append(views, FavoriteView{Favorite: f, ScopeName: s.scopeName(ctx, f.ScopeType, f.ScopeID)})
			continue
		}
		views = append(views, s.view(ctx, f, loadout, viewerProfileID))
	}
	return views, nil
}

// ListForLoadout answers "who vouches for this?" on a loadout page.
//
// Profile bookmarks are excluded: who privately saved your loadout is nobody's business,
// including yours. Only community endorsements are public statements.
func (s *FavoriteService) ListForLoadout(ctx context.Context, loadoutID, viewerProfileID string) ([]FavoriteView, error) {
	loadout, err := s.store.GetLoadout(ctx, loadoutID)
	if err != nil {
		return nil, fmt.Errorf("%w: loadout %s", core.ErrNotFound, loadoutID)
	}
	if !loadout.IsVisibleTo(viewerProfileID) {
		return nil, fmt.Errorf("%w: loadout %s is private", core.ErrForbidden, loadoutID)
	}

	favorites, err := s.store.ListFavoritesForLoadout(ctx, loadoutID)
	if err != nil {
		return nil, err
	}
	views := make([]FavoriteView, 0, len(favorites))
	for _, f := range favorites {
		if f.ScopeType != core.FavoriteCommunity {
			continue
		}
		views = append(views, s.view(ctx, f, loadout, viewerProfileID))
	}
	return views, nil
}

// Get reports one scope's favorite of one loadout, for rendering the endorse button in
// the right state. A miss is (nil, nil) rather than an error.
func (s *FavoriteService) Get(ctx context.Context, viewerProfileID string, scopeType core.FavoriteScope, scopeID, loadoutID string) (*FavoriteView, error) {
	if !scopeType.IsValid() {
		return nil, fmt.Errorf("%w: scope_type must be profile or community", core.ErrInvalid)
	}
	if scopeType == core.FavoriteProfile && scopeID != viewerProfileID {
		return nil, fmt.Errorf("%w: bookmarks are private to their profile", core.ErrForbidden)
	}
	f, err := s.store.GetFavorite(ctx, scopeType, scopeID, loadoutID)
	if err != nil || f == nil {
		return nil, err
	}
	loadout, err := s.store.GetLoadout(ctx, f.LoadoutID)
	if err != nil {
		return &FavoriteView{Favorite: *f, ScopeName: s.scopeName(ctx, f.ScopeType, f.ScopeID)}, nil
	}
	view := s.view(ctx, *f, loadout, viewerProfileID)
	return &view, nil
}

// view decorates a stored favorite with availability, staleness, and the scope's name.
func (s *FavoriteService) view(ctx context.Context, f core.Favorite, loadout core.Loadout, viewerProfileID string) FavoriteView {
	out := FavoriteView{Favorite: f, ScopeName: s.scopeName(ctx, f.ScopeType, f.ScopeID)}
	if !loadout.IsVisibleTo(viewerProfileID) {
		// The owner pulled it private after it was endorsed. The endorsement remains, and
		// remains visibly broken, rather than disappearing without explanation.
		return out
	}
	out.Available = true

	entries, err := s.store.ListLoadoutEntries(ctx, loadout.ID)
	if err == nil {
		// An empty stored fingerprint means the favorite predates fingerprinting, so we
		// cannot tell whether it drifted. Claiming "stale" would be a guess and claiming
		// "fresh" would be a lie; not flagging it is the honest default.
		if f.Fingerprint != "" {
			out.Stale = core.FingerprintLoadout(loadout, entries) != f.Fingerprint
		}
	}
	if summary, err := s.loadouts.Summarize(ctx, loadout, viewerProfileID); err == nil {
		out.Loadout = &summary
	}
	return out
}

// scopeName resolves a scope ID to something a human recognises. A missing entity yields
// an empty string rather than an error: a broken endorsement should still render.
func (s *FavoriteService) scopeName(ctx context.Context, scopeType core.FavoriteScope, scopeID string) string {
	switch scopeType {
	case core.FavoriteCommunity:
		if c, err := s.store.GetCommunity(ctx, scopeID); err == nil {
			return c.Name
		}
	case core.FavoriteProfile:
		if p, err := s.store.GetProfile(ctx, scopeID); err == nil {
			return p.Handle
		}
	}
	return ""
}

// requireScopeAuthority answers "may you speak for this scope?". A profile speaks only for
// itself; a community speaks through its admins.
//
// Note what is absent: the loadout owner's permission. Endorsement is speech about a
// visible object, and the owner retains the only lever that matters — control of the
// object itself.
func (s *FavoriteService) requireScopeAuthority(ctx context.Context, actorProfileID string, scopeType core.FavoriteScope, scopeID string) error {
	if actorProfileID == "" {
		return fmt.Errorf("%w: a profile is required to favorite", core.ErrForbidden)
	}
	if scopeID == "" {
		return fmt.Errorf("%w: scope_id is required", core.ErrInvalid)
	}
	switch scopeType {
	case core.FavoriteProfile:
		if scopeID != actorProfileID {
			return fmt.Errorf("%w: you may only bookmark to your own profile", core.ErrForbidden)
		}
		return nil
	case core.FavoriteCommunity:
		if _, err := s.store.GetCommunity(ctx, scopeID); err != nil {
			return fmt.Errorf("%w: community %s", core.ErrNotFound, scopeID)
		}
		if err := s.community.RequireAdmin(ctx, scopeID, actorProfileID); err != nil {
			// An endorsement carries the community's name, so it takes an admin.
			return err
		}
		return nil
	default:
		return fmt.Errorf("%w: scope_type must be profile or community", core.ErrInvalid)
	}
}

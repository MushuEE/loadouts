package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/gmccloskey/loadouts/backend/internal/core"
	"github.com/gmccloskey/loadouts/backend/internal/db"
)

// CommunityService owns Communities, membership, and the community metadata layer.
type CommunityService struct {
	store db.Store
}

func NewCommunityService(store db.Store) *CommunityService {
	return &CommunityService{store: store}
}

// MemberView pairs a membership with the profile it belongs to, for member lists.
type MemberView struct {
	Membership core.CommunityMembership `json:"membership"`
	Profile    core.Profile             `json:"profile"`
}

// Create registers a community and makes the creating profile its owner.
func (s *CommunityService) Create(ctx context.Context, actorProfileID string, c core.Community) (core.Community, error) {
	if actorProfileID == "" {
		return core.Community{}, fmt.Errorf("%w: a profile is required to create a community", core.ErrForbidden)
	}
	if strings.TrimSpace(c.Name) == "" {
		return core.Community{}, fmt.Errorf("%w: name is required", core.ErrInvalid)
	}

	slug := core.Slugify(c.Slug)
	if slug == "" {
		slug = core.Slugify(c.Name)
	}
	if _, err := s.store.GetCommunityBySlug(ctx, slug); err == nil {
		return core.Community{}, fmt.Errorf("%w: community /%s already exists", core.ErrConflict, slug)
	}

	c.ID = core.NewID("cmy")
	c.Slug = slug
	c.CreatedBy = actorProfileID
	c.MemberCount = 0
	if c.MetadataHint == nil {
		c.MetadataHint = core.Metadata{}
	}

	if err := s.store.CreateCommunity(ctx, c); err != nil {
		return core.Community{}, err
	}
	if err := s.store.UpsertMembership(ctx, core.CommunityMembership{
		CommunityID: c.ID,
		ProfileID:   actorProfileID,
		Role:        core.RoleOwner,
	}); err != nil {
		return core.Community{}, err
	}

	return s.store.GetCommunity(ctx, c.ID)
}

// Get resolves a community by ID or slug.
func (s *CommunityService) Get(ctx context.Context, idOrSlug string) (core.Community, error) {
	if c, err := s.store.GetCommunity(ctx, idOrSlug); err == nil {
		return c, nil
	}
	c, err := s.store.GetCommunityBySlug(ctx, idOrSlug)
	if err != nil {
		return core.Community{}, fmt.Errorf("%w: community %s", core.ErrNotFound, idOrSlug)
	}
	return c, nil
}

func (s *CommunityService) List(ctx context.Context, query string) ([]core.Community, error) {
	return s.store.ListCommunities(ctx, query)
}

// Join adds a profile as a plain member (idempotent, and never demotes an existing role).
func (s *CommunityService) Join(ctx context.Context, communityID, profileID string) (core.CommunityMembership, error) {
	if existing, err := s.store.GetMembership(ctx, communityID, profileID); err == nil {
		return *existing, nil
	}
	m := core.CommunityMembership{
		CommunityID: communityID,
		ProfileID:   profileID,
		Role:        core.RoleMember,
	}
	if err := s.store.UpsertMembership(ctx, m); err != nil {
		return core.CommunityMembership{}, err
	}
	return m, nil
}

// Leave removes a membership. The last owner may not abandon the community.
func (s *CommunityService) Leave(ctx context.Context, communityID, profileID string) error {
	membership, err := s.store.GetMembership(ctx, communityID, profileID)
	if err != nil {
		return fmt.Errorf("%w: not a member", core.ErrNotFound)
	}
	if membership.Role == core.RoleOwner {
		owners, err := s.countOwners(ctx, communityID)
		if err != nil {
			return err
		}
		if owners <= 1 {
			return fmt.Errorf("%w: transfer ownership before leaving", core.ErrForbidden)
		}
	}
	return s.store.DeleteMembership(ctx, communityID, profileID)
}

func (s *CommunityService) countOwners(ctx context.Context, communityID string) (int, error) {
	members, err := s.store.ListMemberships(ctx, communityID, "")
	if err != nil {
		return 0, err
	}
	count := 0
	for _, m := range members {
		if m.Role == core.RoleOwner {
			count++
		}
	}
	return count, nil
}

// Members lists memberships joined with their profiles.
func (s *CommunityService) Members(ctx context.Context, communityID string) ([]MemberView, error) {
	memberships, err := s.store.ListMemberships(ctx, communityID, "")
	if err != nil {
		return nil, err
	}
	views := make([]MemberView, 0, len(memberships))
	for _, m := range memberships {
		profile, err := s.store.GetProfile(ctx, m.ProfileID)
		if err != nil {
			continue // Tolerate dangling memberships rather than failing the whole list.
		}
		views = append(views, MemberView{Membership: m, Profile: profile})
	}
	return views, nil
}

// ListForProfile returns the communities a profile belongs to.
func (s *CommunityService) ListForProfile(ctx context.Context, profileID string) ([]core.Community, error) {
	memberships, err := s.store.ListMemberships(ctx, "", profileID)
	if err != nil {
		return nil, err
	}
	communities := make([]core.Community, 0, len(memberships))
	for _, m := range memberships {
		c, err := s.store.GetCommunity(ctx, m.CommunityID)
		if err != nil {
			continue
		}
		communities = append(communities, c)
	}
	return communities, nil
}

// RoleOf reports a profile's role, or "" if they are not a member.
func (s *CommunityService) RoleOf(ctx context.Context, communityID, profileID string) core.MemberRole {
	if profileID == "" {
		return ""
	}
	m, err := s.store.GetMembership(ctx, communityID, profileID)
	if err != nil {
		return ""
	}
	return m.Role
}

// RequireAdmin is the single gate for community-owned mutations.
func (s *CommunityService) RequireAdmin(ctx context.Context, communityID, profileID string) error {
	if !s.RoleOf(ctx, communityID, profileID).CanAdminister() {
		return fmt.Errorf("%w: admin role required in this community", core.ErrForbidden)
	}
	return nil
}

// SetItemLayer writes the community's metadata layer for an item, e.g. the UL Backpacking
// community adding {ul_score, comfort, durability} to a tent. Admin-gated.
func (s *CommunityService) SetItemLayer(ctx context.Context, actorProfileID, communityID, itemID string, metadata core.Metadata) (core.CommunityItemLayer, error) {
	if err := s.RequireAdmin(ctx, communityID, actorProfileID); err != nil {
		return core.CommunityItemLayer{}, err
	}
	if _, err := s.store.GetItem(ctx, itemID); err != nil {
		return core.CommunityItemLayer{}, fmt.Errorf("%w: item %s", core.ErrNotFound, itemID)
	}
	if metadata == nil {
		metadata = core.Metadata{}
	}

	layer := core.CommunityItemLayer{
		CommunityID: communityID,
		ItemID:      itemID,
		Metadata:    metadata,
		UpdatedBy:   actorProfileID,
	}
	if err := s.store.UpsertCommunityItemLayer(ctx, layer); err != nil {
		return core.CommunityItemLayer{}, err
	}
	return layer, nil
}

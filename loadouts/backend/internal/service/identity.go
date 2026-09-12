package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/gmccloskey/loadouts/backend/internal/core"
	"github.com/gmccloskey/loadouts/backend/internal/db"
)

// IdentityService owns Users and their Profiles.
//
// A User is the account root; a Profile is the public persona that authors everything.
// One User may hold several Profiles (e.g. a backpacking persona and a streetwear persona).
type IdentityService struct {
	store db.Store
}

func NewIdentityService(store db.Store) *IdentityService {
	return &IdentityService{store: store}
}

func (s *IdentityService) CreateUser(ctx context.Context, email, displayName string) (core.User, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" {
		return core.User{}, fmt.Errorf("%w: email is required", core.ErrInvalid)
	}
	if displayName == "" {
		displayName = strings.Split(email, "@")[0]
	}

	user := core.User{
		ID:          core.NewID("usr"),
		Email:       email,
		DisplayName: displayName,
	}
	if err := s.store.CreateUser(ctx, user); err != nil {
		return core.User{}, err
	}
	return user, nil
}

func (s *IdentityService) GetUser(ctx context.Context, id string) (core.User, error) {
	user, err := s.store.GetUser(ctx, id)
	if err != nil {
		return core.User{}, fmt.Errorf("%w: user %s", core.ErrNotFound, id)
	}
	return user, nil
}

func (s *IdentityService) ListUsers(ctx context.Context) ([]core.User, error) {
	return s.store.ListUsers(ctx)
}

// CreateProfile attaches a new persona to an existing user.
func (s *IdentityService) CreateProfile(ctx context.Context, profile core.Profile) (core.Profile, error) {
	if profile.UserID == "" {
		return core.Profile{}, fmt.Errorf("%w: user_id is required", core.ErrInvalid)
	}
	if _, err := s.store.GetUser(ctx, profile.UserID); err != nil {
		return core.Profile{}, fmt.Errorf("%w: user %s", core.ErrNotFound, profile.UserID)
	}

	handle := core.Slugify(profile.Handle)
	if handle == "" {
		return core.Profile{}, fmt.Errorf("%w: handle is required", core.ErrInvalid)
	}
	if _, err := s.store.GetProfileByHandle(ctx, handle); err == nil {
		return core.Profile{}, fmt.Errorf("%w: handle @%s is taken", core.ErrConflict, handle)
	}

	profile.ID = core.NewID("prf")
	profile.Handle = handle
	if profile.DisplayName == "" {
		profile.DisplayName = handle
	}
	if err := s.store.CreateProfile(ctx, profile); err != nil {
		return core.Profile{}, err
	}
	return profile, nil
}

func (s *IdentityService) GetProfile(ctx context.Context, id string) (core.Profile, error) {
	profile, err := s.store.GetProfile(ctx, id)
	if err != nil {
		return core.Profile{}, fmt.Errorf("%w: profile %s", core.ErrNotFound, id)
	}
	return profile, nil
}

// Resolve looks up a profile by ID first, then by handle. Handy for URLs and the dev-auth
// header, where callers naturally use whichever identifier they have.
func (s *IdentityService) Resolve(ctx context.Context, idOrHandle string) (core.Profile, error) {
	if idOrHandle == "" {
		return core.Profile{}, fmt.Errorf("%w: no profile identifier", core.ErrNotFound)
	}
	if profile, err := s.store.GetProfile(ctx, idOrHandle); err == nil {
		return profile, nil
	}
	profile, err := s.store.GetProfileByHandle(ctx, strings.TrimPrefix(idOrHandle, "@"))
	if err != nil {
		return core.Profile{}, fmt.Errorf("%w: profile %s", core.ErrNotFound, idOrHandle)
	}
	return profile, nil
}

func (s *IdentityService) ListProfiles(ctx context.Context, userID string) ([]core.Profile, error) {
	return s.store.ListProfiles(ctx, userID)
}

// Package auth provides Day 0 request authentication.
//
// There are no passwords or sessions yet: the client asserts which Profile it is acting as
// via the X-Profile-ID header (an ID or a @handle). This is deliberately the *only* place
// that decides "who is calling", so swapping in real auth later is a single-file change.
package auth

import (
	"context"
	"net/http"

	"github.com/gmccloskey/loadouts/backend/internal/core"
	"github.com/gmccloskey/loadouts/backend/internal/service"
)

type contextKey string

const profileContextKey contextKey = "loadouts.profile"

// HeaderProfile is the primary actor header; HeaderUser is accepted for backwards
// compatibility with the original inventory endpoints.
const (
	HeaderProfile = "X-Profile-ID"
	HeaderUser    = "X-User-ID"
)

// Middleware resolves the acting profile and attaches it to the request context.
// Requests without a resolvable profile are *not* rejected here; they simply act
// anonymously, and individual handlers decide whether that is acceptable.
func Middleware(identity *service.IdentityService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			identifier := r.Header.Get(HeaderProfile)
			if identifier == "" {
				identifier = r.Header.Get(HeaderUser)
			}

			if identifier != "" {
				if profile, err := identity.Resolve(r.Context(), identifier); err == nil {
					ctx := context.WithValue(r.Context(), profileContextKey, profile)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// CurrentProfile returns the acting profile, if any.
func CurrentProfile(ctx context.Context) (core.Profile, bool) {
	profile, ok := ctx.Value(profileContextKey).(core.Profile)
	return profile, ok
}

// ProfileID returns the acting profile's ID, or "" for an anonymous caller.
func ProfileID(ctx context.Context) string {
	profile, ok := CurrentProfile(ctx)
	if !ok {
		return ""
	}
	return profile.ID
}

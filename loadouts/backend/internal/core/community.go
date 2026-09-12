package core

import "time"

// MemberRole describes a Profile's authority inside a Community.
type MemberRole string

const (
	RoleMember MemberRole = "member"
	RoleAdmin  MemberRole = "admin"
	RoleOwner  MemberRole = "owner"
)

// CanAdminister reports whether the role may mutate community-owned data
// (community item layers, community templates, pinned loadouts).
func (r MemberRole) CanAdminister() bool {
	return r == RoleAdmin || r == RoleOwner
}

// Community is a group layer above Profiles (think subreddits): Backpacking, UL Backpacking,
// WallStreetBets, WoW, NYC Fashion. Communities host templates, loadouts, and their own
// metadata layer on top of global items.
type Community struct {
	ID           string    `json:"id" db:"id"`
	Slug         string    `json:"slug" db:"slug"`
	Name         string    `json:"name" db:"name"`
	Description  string    `json:"description" db:"description"`
	CreatedBy    string    `json:"created_by" db:"created_by"` // Profile ID
	MemberCount  int       `json:"member_count" db:"member_count"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	MetadataHint Metadata  `json:"metadata_hint" db:"metadata_hint"` // Suggested layer keys, e.g. {"ul_score": "number"}
}

// CommunityMembership links a Profile to a Community with a role.
type CommunityMembership struct {
	CommunityID string     `json:"community_id" db:"community_id"`
	ProfileID   string     `json:"profile_id" db:"profile_id"`
	Role        MemberRole `json:"role" db:"role"`
	JoinedAt    time.Time  `json:"joined_at" db:"joined_at"`
}

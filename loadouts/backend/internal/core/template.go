package core

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

// OwnerType identifies who owns a Template or a Loadout.
type OwnerType string

const (
	OwnerPlatform  OwnerType = "platform"
	OwnerProfile   OwnerType = "profile"
	OwnerCommunity OwnerType = "community"
)

// FreeformTemplateID is the built-in, always-available template that imposes no structure.
// It is the "base generic template to generally add any items without structure".
const FreeformTemplateID = "platform-freeform"

// SlotDefinition is a single position in a Template (or a nested container Item).
// Example: {id: "shelter", name: "Tent", accepted_categories: ["shelter"], required: true}
type SlotDefinition struct {
	ID                 string   `json:"id"`
	Name               string   `json:"name"`
	Description        string   `json:"description,omitempty"`
	AcceptedCategories []string `json:"accepted_categories"`
	Required           bool     `json:"required"`
	MaxItems           int      `json:"max_items"` // 0 == 1 (single item), -1 == unlimited
	Position           int      `json:"position"`
}

// Accepts reports whether an item category may occupy this slot.
// The "universal" pseudo-category (on either side) matches anything.
func (s SlotDefinition) Accepts(category string) bool {
	if len(s.AcceptedCategories) == 0 {
		return true
	}
	for _, c := range s.AcceptedCategories {
		if c == "universal" || c == category {
			return true
		}
	}
	return false
}

// Capacity normalizes MaxItems into an effective capacity. -1 means unlimited.
func (s SlotDefinition) Capacity() int {
	if s.MaxItems == 0 {
		return 1
	}
	return s.MaxItems
}

// SlotList is a JSONB-backed slice of SlotDefinition.
type SlotList []SlotDefinition

func (s SlotList) Value() (driver.Value, error) {
	if s == nil {
		return []byte("[]"), nil
	}
	return json.Marshal(s)
}

func (s *SlotList) Scan(value interface{}) error {
	if value == nil {
		*s = SlotList{}
		return nil
	}
	b, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("SlotList: type assertion to []byte failed")
	}
	return json.Unmarshal(b, s)
}

// Template is the foundation and scaffolding for building a Loadout.
// The Template record holds identity and ownership; the *shape* lives in immutable
// TemplateVersions so that publishing a change can never break someone else's Loadout.
type Template struct {
	ID            string    `json:"id" db:"id"`
	Name          string    `json:"name" db:"name"`
	Description   string    `json:"description" db:"description"`
	OwnerType     OwnerType `json:"owner_type" db:"owner_type"`
	OwnerID       string    `json:"owner_id" db:"owner_id"`         // Profile ID or Community ID; empty for platform
	CommunityID   string    `json:"community_id" db:"community_id"` // Optional hosting community
	LatestVersion int       `json:"latest_version" db:"latest_version"`
	IsPublic      bool      `json:"is_public" db:"is_public"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time `json:"updated_at" db:"updated_at"`
}

// TemplateVersion is an immutable snapshot of a Template's slot structure.
type TemplateVersion struct {
	TemplateID string    `json:"template_id" db:"template_id"`
	Version    int       `json:"version" db:"version"`
	Slots      SlotList  `json:"slots" db:"slots"`
	Changelog  string    `json:"changelog" db:"changelog"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
}

// SlotByID finds a slot in this version.
func (tv TemplateVersion) SlotByID(id string) (SlotDefinition, bool) {
	for _, s := range tv.Slots {
		if s.ID == id {
			return s, true
		}
	}
	return SlotDefinition{}, false
}

// TemplateDetail bundles a template with a specific (usually latest) version for API responses.
type TemplateDetail struct {
	Template Template        `json:"template"`
	Version  TemplateVersion `json:"version"`
	Versions []int           `json:"versions,omitempty"`
}

// TemplateQuery filters template listings.
type TemplateQuery struct {
	Text        string
	OwnerType   OwnerType
	OwnerID     string
	CommunityID string
	OnlyPublic  bool
}

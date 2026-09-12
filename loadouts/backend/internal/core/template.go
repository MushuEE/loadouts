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

// SelectionMode describes how several children in one slot combine into the parent's
// totals. It only has meaning for a slot that can hold more than one thing.
//
// The distinction is not cosmetic. Three days of meals in a food slot are all carried, so
// a total that ignored two of them would be a lie. Three candidate tents in a "shortlist"
// slot are alternatives you are choosing between, so adding them together would be
// nonsense. The same swipe-through UI serves both; only the arithmetic differs.
type SelectionMode string

const (
	// SelectionSum counts every child. The default, and what an unset field means.
	SelectionSum SelectionMode = "sum"
	// SelectionAlternatives counts only the child marked Selected.
	SelectionAlternatives SelectionMode = "alternatives"
)

// IsValid reports whether the mode is one the rollup understands.
func (m SelectionMode) IsValid() bool {
	return m == "" || m == SelectionSum || m == SelectionAlternatives
}

// Normalize turns the zero value into the default.
func (m SelectionMode) Normalize() SelectionMode {
	if m == "" {
		return SelectionSum
	}
	return m
}

// SlotDefinition is a single position in a Template (or a nested container Item).
// Example: {id: "shelter", name: "Tent", accepted_categories: ["shelter"], required: true}
//
// A slot holds either items or sub-loadouts, never both. An item slot lists
// AcceptedCategories; a sub-loadout slot lists AcceptedTemplateIDs. Allowing both at once
// would make "what goes here?" unanswerable in the UI, so ValidateSlots rejects it.
type SlotDefinition struct {
	ID                 string   `json:"id"`
	Name               string   `json:"name"`
	Description        string   `json:"description,omitempty"`
	AcceptedCategories []string `json:"accepted_categories"`
	Required           bool     `json:"required"`
	MaxItems           int      `json:"max_items"` // 0 == 1 (single item), -1 == unlimited
	Position           int      `json:"position"`

	// AcceptedTemplateIDs makes this a sub-loadout slot: a "Food" slot that accepts
	// loadouts built on the "Meals" template. Empty with IsSubLoadout set means any
	// template is allowed.
	AcceptedTemplateIDs []string `json:"accepted_template_ids,omitempty"`
	// IsSubLoadout marks the slot as holding loadouts even when any template is allowed,
	// which an empty AcceptedTemplateIDs could not otherwise express.
	IsSubLoadout bool `json:"is_sub_loadout,omitempty"`
	// Selection controls how multiple children roll up. Empty means SelectionSum.
	Selection SelectionMode `json:"selection,omitempty"`
}

// HoldsSubLoadouts reports whether this slot takes loadouts rather than items.
func (s SlotDefinition) HoldsSubLoadouts() bool {
	return s.IsSubLoadout || len(s.AcceptedTemplateIDs) > 0
}

// AcceptsTemplate reports whether a loadout built on templateID may occupy this slot.
func (s SlotDefinition) AcceptsTemplate(templateID string) bool {
	if !s.HoldsSubLoadouts() {
		return false
	}
	if len(s.AcceptedTemplateIDs) == 0 {
		return true
	}
	for _, id := range s.AcceptedTemplateIDs {
		if id == templateID {
			return true
		}
	}
	return false
}

// Accepts reports whether an item category may occupy this slot.
// The "universal" pseudo-category (on either side) matches anything.
func (s SlotDefinition) Accepts(category string) bool {
	if s.HoldsSubLoadouts() {
		return false
	}
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

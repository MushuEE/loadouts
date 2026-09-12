package core

import "time"

// Visibility controls who can see a Loadout.
type Visibility string

const (
	VisibilityPrivate  Visibility = "private"  // Owner only
	VisibilityUnlisted Visibility = "unlisted" // Anyone with the link
	VisibilityPublic   Visibility = "public"   // Appears in discover / community feeds
)

// LoadoutStatus separates a work-in-progress draft from a shared loadout.
type LoadoutStatus string

const (
	StatusDraft     LoadoutStatus = "draft"
	StatusPublished LoadoutStatus = "published"
)

// Loadout is the marriage of a Template and Items: the core shareable entity.
type Loadout struct {
	ID              string        `json:"id" db:"id"`
	Name            string        `json:"name" db:"name"`
	Description     string        `json:"description" db:"description"`
	OwnerProfileID  string        `json:"owner_profile_id" db:"owner_profile_id"`
	CommunityID     string        `json:"community_id" db:"community_id"` // Optional: the community this loadout is posted to
	TemplateID      string        `json:"template_id" db:"template_id"`
	TemplateVersion int           `json:"template_version" db:"template_version"`
	Visibility      Visibility    `json:"visibility" db:"visibility"`
	Status          LoadoutStatus `json:"status" db:"status"`
	ForkedFrom      string        `json:"forked_from" db:"forked_from"`
	CoverImageURL   string        `json:"cover_image_url" db:"cover_image_url"`
	ForkCount       int           `json:"fork_count" db:"fork_count"`
	CreatedAt       time.Time     `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time     `json:"updated_at" db:"updated_at"`
}

// IsVisibleTo reports whether a viewer profile may read this loadout.
// An empty viewerProfileID means an anonymous visitor.
func (l Loadout) IsVisibleTo(viewerProfileID string) bool {
	if l.OwnerProfileID == viewerProfileID && viewerProfileID != "" {
		return true
	}
	return l.Visibility == VisibilityPublic || l.Visibility == VisibilityUnlisted
}

// LoadoutEntry is one item placed into one slot of a Loadout.
// ParentEntryID enables the telescoping/nesting UX: an item that provides its own slots
// (a backpack, a ditty bag, a pot) can hold further entries whose SlotID refers to the
// parent item's ProvidedSlots rather than the template's slots.
type LoadoutEntry struct {
	ID            string    `json:"id" db:"id"`
	LoadoutID     string    `json:"loadout_id" db:"loadout_id"`
	SlotID        string    `json:"slot_id" db:"slot_id"`
	ParentEntryID string    `json:"parent_entry_id" db:"parent_entry_id"`
	ItemID        string    `json:"item_id" db:"item_id"`
	Quantity      int       `json:"quantity" db:"quantity"`
	Note          string    `json:"note" db:"note"`
	Position      int       `json:"position" db:"position"` // Grid index (0-31) for the client grid
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
}

// LoadoutStats are the recursive rollups shown in the UI.
// BaseWeightG excludes consumables (food, water, fuel) — the metric hikers actually care about.
type LoadoutStats struct {
	TotalWeightG      float64 `json:"total_weight_g"`
	BaseWeightG       float64 `json:"base_weight_g"`
	ConsumableWeightG float64 `json:"consumable_weight_g"`
	TotalCostCents    float64 `json:"total_cost_cents"`
	ItemCount         int     `json:"item_count"`
}

// ValidationIssue is a soft or hard problem with a Loadout relative to its TemplateVersion.
type ValidationIssue struct {
	SlotID   string `json:"slot_id"`
	EntryID  string `json:"entry_id,omitempty"`
	Severity string `json:"severity"` // "error" | "warning"
	Message  string `json:"message"`
}

// ResolvedEntry is a LoadoutEntry with its item fully resolved through the metadata layers.
type ResolvedEntry struct {
	Entry    LoadoutEntry    `json:"entry"`
	Item     ResolvedItem    `json:"item"`
	Children []ResolvedEntry `json:"children,omitempty"`
}

// LoadoutDetail is the full read model served for a single loadout.
type LoadoutDetail struct {
	Loadout  Loadout           `json:"loadout"`
	Owner    Profile           `json:"owner"`
	Template TemplateDetail    `json:"template"`
	Entries  []ResolvedEntry   `json:"entries"`
	Stats    LoadoutStats      `json:"stats"`
	Issues   []ValidationIssue `json:"issues"`
}

// LoadoutSummary is the lightweight card shown in feeds and lists.
type LoadoutSummary struct {
	Loadout      Loadout      `json:"loadout"`
	OwnerHandle  string       `json:"owner_handle"`
	OwnerName    string       `json:"owner_name"`
	TemplateName string       `json:"template_name"`
	CommunityID  string       `json:"community_id,omitempty"`
	Stats        LoadoutStats `json:"stats"`
	ItemPreview  []string     `json:"item_preview"` // First few item names
}

// DiscoverQuery filters loadout listings (public feed, community feed, profile shelf).
type DiscoverQuery struct {
	Text        string
	CommunityID string
	TemplateID  string
	ProfileID   string
	Status      LoadoutStatus // Empty means any
	OnlyPublic  bool
	Limit       int
}

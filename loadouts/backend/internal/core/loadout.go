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

// LoadoutEntry is one item — or one sub-loadout — placed into one slot of a Loadout.
//
// ParentEntryID enables the telescoping/nesting UX: an item that provides its own slots
// (a backpack, a ditty bag, a pot) can hold further entries whose SlotID refers to the
// parent item's ProvidedSlots rather than the template's slots.
//
// ChildLoadoutID is a different axis entirely. It points at another Loadout — a "Meals
// Day 1" hanging off a trip's food slot — which is an independent, publishable, forkable
// object rather than a part of this one. An entry is exactly one of the two kinds:
// ItemID set, or ChildLoadoutID set, never both and never neither.
type LoadoutEntry struct {
	ID            string `json:"id" db:"id"`
	LoadoutID     string `json:"loadout_id" db:"loadout_id"`
	SlotID        string `json:"slot_id" db:"slot_id"`
	ParentEntryID string `json:"parent_entry_id" db:"parent_entry_id"`
	ItemID        string `json:"item_id" db:"item_id"`
	// ChildLoadoutID references another loadout occupying this slot.
	ChildLoadoutID string `json:"child_loadout_id,omitempty" db:"child_loadout_id"`
	// Selected marks the active child in an "alternatives" slot. It is ignored by
	// "sum" slots, where everything counts. Selection is an explicit field rather than
	// "whichever sorts first" so that reordering the UI cannot silently change a total.
	Selected  bool      `json:"selected" db:"selected"`
	Quantity  int       `json:"quantity" db:"quantity"`
	Note      string    `json:"note" db:"note"`
	Position  int       `json:"position" db:"position"` // Grid index (0-31) for the client grid
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// IsSubLoadout reports whether this entry references another loadout rather than an item.
func (e LoadoutEntry) IsSubLoadout() bool { return e.ChildLoadoutID != "" }

// EffectiveQuantity normalizes a missing or nonsensical quantity to 1.
func (e LoadoutEntry) EffectiveQuantity() int {
	if e.Quantity <= 0 {
		return 1
	}
	return e.Quantity
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
//
// Children and SubLoadout are deliberately separate fields because they nest along
// different axes. Children is a pot inside a pack inside *this* loadout. SubLoadout is a
// reference to a different loadout entirely, with its own owner, template, and visibility.
type ResolvedEntry struct {
	Entry      LoadoutEntry        `json:"entry"`
	Item       ResolvedItem        `json:"item"`
	Children   []ResolvedEntry     `json:"children,omitempty"`
	SubLoadout *ResolvedSubLoadout `json:"sub_loadout,omitempty"`
}

// ResolvedSubLoadout is a referenced loadout as seen from its parent.
//
// A sub-loadout the viewer may not read still reports its Stats. That is a deliberate
// trade: the alternative is a parent whose displayed weight silently omits part of the
// pack, which is worse than admitting something is there. The contents, the name, and the
// owner stay hidden. Attachment is restricted to loadouts the parent's owner owns, so the
// only aggregate ever disclosed this way is the discloser's own.
type ResolvedSubLoadout struct {
	LoadoutID string `json:"loadout_id"`
	// Name is empty when the viewer may not read the child.
	Name string `json:"name,omitempty"`
	// Visible reports whether the viewer may read the child; false means render a
	// placeholder that still admits to a weight.
	Visible bool `json:"visible"`
	// Missing reports that the referenced loadout no longer exists.
	Missing bool `json:"missing,omitempty"`
	// Counted reports whether this child contributed to the parent's totals. False for
	// the unselected options of an "alternatives" slot.
	Counted    bool            `json:"counted"`
	TemplateID string          `json:"template_id,omitempty"`
	Stats      LoadoutStats    `json:"stats"`
	Entries    []ResolvedEntry `json:"entries,omitempty"`
	// Depth is this node's distance from the root loadout, starting at 1.
	Depth int `json:"depth"`
}

// Add accumulates another set of stats, scaled by a quantity.
func (s *LoadoutStats) Add(other LoadoutStats, times int) {
	if times <= 0 {
		times = 1
	}
	n := float64(times)
	s.TotalWeightG += other.TotalWeightG * n
	s.BaseWeightG += other.BaseWeightG * n
	s.ConsumableWeightG += other.ConsumableWeightG * n
	s.TotalCostCents += other.TotalCostCents * n
	s.ItemCount += other.ItemCount * times
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

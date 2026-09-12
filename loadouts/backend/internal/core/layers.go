package core

import (
	"fmt"
	"time"
)

// Layer names used for provenance reporting on a ResolvedItem.
const (
	LayerGlobal      = "global"
	LayerCommunity   = "community"
	LayerUserPublic  = "user_public"
	LayerUserPrivate = "user_private"
)

// CoreNamespace is the canonical, platform-owned metadata namespace every item is expected
// to populate. It backs the universal rollups (weight, cost, consumable) that the loadout
// stats engine understands, regardless of hobby.
const CoreNamespace = "core"

// Well-known keys inside CoreNamespace.
const (
	KeyWeightG     = "weight_g"
	KeyCostCents   = "cost_cents"
	KeyConsumable  = "consumable"
	KeyDescription = "description"
)

// CommunityItemLayer is metadata a Community attaches to a global Item. It is public, but
// only surfaces when the item is viewed in that community's context.
//
//	Example (UltraLight Backpacking): {"ul_backpacking": {"ul_score": 7.8, "comfort": 8.1}}
type CommunityItemLayer struct {
	CommunityID string    `json:"community_id" db:"community_id"`
	ItemID      string    `json:"item_id" db:"item_id"`
	Metadata    Metadata  `json:"metadata" db:"metadata"`
	UpdatedBy   string    `json:"updated_by" db:"updated_by"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

// ProfileItemLayer is metadata a Profile attaches to an Item.
//   - PublicMetadata: overrides/extensions visible to everyone (e.g. filling in a missing
//     'material' the global item never specified).
//   - PrivateMetadata: notes and custom fields only the owning profile can read.
type ProfileItemLayer struct {
	ProfileID       string    `json:"profile_id" db:"profile_id"`
	ItemID          string    `json:"item_id" db:"item_id"`
	CustomImageURL  string    `json:"custom_image_url" db:"custom_image_url"`
	PublicMetadata  Metadata  `json:"public_metadata" db:"public_metadata"`
	PrivateMetadata Metadata  `json:"private_metadata" db:"private_metadata"`
	UpdatedAt       time.Time `json:"updated_at" db:"updated_at"`
}

// LayerContext describes *who* is looking at an item and *where*, which determines
// which layers apply and whether private data may be revealed.
type LayerContext struct {
	ViewerProfileID string // Empty for anonymous
	OwnerProfileID  string // Whose user-layer to apply; defaults to the viewer
	CommunityID     string // Optional community scope
}

// Owner returns the profile whose user layer should be applied.
func (c LayerContext) Owner() string {
	if c.OwnerProfileID != "" {
		return c.OwnerProfileID
	}
	return c.ViewerProfileID
}

// CanSeePrivate reports whether the private layer may be included.
func (c LayerContext) CanSeePrivate() bool {
	return c.ViewerProfileID != "" && c.ViewerProfileID == c.Owner()
}

// ResolvedItem is a global Item flattened through every applicable layer, plus the
// provenance map so the UI can explain where each value came from.
type ResolvedItem struct {
	ID            string                 `json:"id"`
	Name          string                 `json:"name"`
	Category      string                 `json:"category"`
	ImageURL      string                 `json:"image_url"`
	ProvidedSlots SlotList               `json:"provided_slots"`
	Metadata      map[string]interface{} `json:"metadata"`
	Provenance    map[string]string      `json:"provenance"` // "namespace.key" -> layer name
	AppliedLayers []string               `json:"applied_layers"`
	Sources       []ResolvedSource       `json:"sources,omitempty"`
}

// WeightG returns the resolved core weight in grams.
func (r ResolvedItem) WeightG() float64 { return r.numeric(KeyWeightG) }

// CostCents returns the resolved core cost in cents.
func (r ResolvedItem) CostCents() float64 { return r.numeric(KeyCostCents) }

// IsConsumable reports the resolved core consumable flag.
func (r ResolvedItem) IsConsumable() bool {
	core, ok := r.Metadata[CoreNamespace].(map[string]interface{})
	if !ok {
		return false
	}
	b, _ := core[KeyConsumable].(bool)
	return b
}

func (r ResolvedItem) numeric(key string) float64 {
	core, ok := r.Metadata[CoreNamespace].(map[string]interface{})
	if !ok {
		return 0
	}
	switch v := core[key].(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	default:
		return 0
	}
}

// LayerInput bundles the optional layers for a single resolution pass.
type LayerInput struct {
	Community *CommunityItemLayer
	Profile   *ProfileItemLayer
}

// ResolveLayers flattens an Item through the four-layer model:
//
//	global -> community -> user public -> user private (owner only)
//
// Later layers deep-merge over earlier ones. Provenance records the winning layer for every
// leaf key so the client can render "overridden by your community" style affordances.
func ResolveLayers(item Item, in LayerInput, ctx LayerContext) ResolvedItem {
	resolved := ResolvedItem{
		ID:            item.ID,
		Name:          item.Name,
		Category:      item.Category,
		ImageURL:      item.ImageURL,
		ProvidedSlots: item.ProvidedSlots,
		Metadata:      map[string]interface{}{},
		Provenance:    map[string]string{},
		AppliedLayers: []string{LayerGlobal},
	}

	applyLayer(&resolved, item.BaseMetadata, LayerGlobal)

	if in.Community != nil && ctx.CommunityID != "" && in.Community.CommunityID == ctx.CommunityID {
		applyLayer(&resolved, in.Community.Metadata, LayerCommunity)
		resolved.AppliedLayers = append(resolved.AppliedLayers, LayerCommunity)
	}

	if in.Profile != nil {
		applyLayer(&resolved, in.Profile.PublicMetadata, LayerUserPublic)
		resolved.AppliedLayers = append(resolved.AppliedLayers, LayerUserPublic)

		if in.Profile.CustomImageURL != "" {
			resolved.ImageURL = in.Profile.CustomImageURL
		}

		// The private layer is redacted for anyone but the owning profile.
		if ctx.CanSeePrivate() && len(in.Profile.PrivateMetadata) > 0 {
			applyLayer(&resolved, in.Profile.PrivateMetadata, LayerUserPrivate)
			resolved.AppliedLayers = append(resolved.AppliedLayers, LayerUserPrivate)
		}
	}

	return resolved
}

// applyLayer deep-merges a namespaced metadata layer into the resolved item and records
// provenance for every leaf key it wins.
func applyLayer(resolved *ResolvedItem, layer Metadata, layerName string) {
	for namespace, raw := range layer {
		value := deepCopy(raw)

		incoming, incomingIsMap := value.(map[string]interface{})
		existing, existingIsMap := resolved.Metadata[namespace].(map[string]interface{})

		if incomingIsMap && existingIsMap {
			resolved.Metadata[namespace] = deepMergeMaps(existing, incoming)
		} else {
			resolved.Metadata[namespace] = value
		}

		if incomingIsMap {
			recordProvenance(resolved.Provenance, namespace, incoming, layerName)
		} else {
			resolved.Provenance[namespace] = layerName
		}
	}
}

func recordProvenance(prov map[string]string, prefix string, m map[string]interface{}, layerName string) {
	for k, v := range m {
		path := fmt.Sprintf("%s.%s", prefix, k)
		if nested, ok := v.(map[string]interface{}); ok {
			recordProvenance(prov, path, nested, layerName)
			continue
		}
		prov[path] = layerName
	}
}

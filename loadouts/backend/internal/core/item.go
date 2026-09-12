package core

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

// SchemaDefinition represents a validation schema for a plugin's metadata.
// For now, we'll store the schema as a JSON raw message (e.g., JSON Schema).
type SchemaDefinition struct {
	ID          string          `json:"id"`
	Version     string          `json:"version"`
	Name        string          `json:"name"`
	Author      string          `json:"author"`
	Definition  json.RawMessage `json:"definition"` // JSON Schema or similar
	CreatedAt   time.Time       `json:"created_at"`
	IsImmutable bool            `json:"is_immutable"`
}

// Item represents the global base item in the metadata store.
// Global items are individual products (physical or virtual); they are public and
// treated as immutable once published. Everything hobby-specific lives in the
// community and profile layers on top (see layers.go).
type Item struct {
	ID   string `json:"id" db:"id"`
	Name string `json:"name" db:"name"`
	// Category drives template slot compatibility (e.g. "pack", "shelter", "sleep").
	Category string `json:"category" db:"category"`
	ImageURL string `json:"image_url" db:"image_url"`
	// ProvidedSlots lets an item act as a container (a pack provides pockets, a pot
	// provides an interior), which is what powers the telescoping loadout view.
	ProvidedSlots SlotList `json:"provided_slots" db:"provided_slots"`
	BaseMetadata  Metadata `json:"base_metadata" db:"base_metadata"` // Namespaced by SchemaID
	// Origin records how the item entered the catalog (OriginCurated / OriginImport).
	Origin string `json:"origin" db:"origin"`
	// Verified marks an item a human has vouched for. Retailer imports land unverified so
	// scraped data can be trusted less than hand-curated data without being hidden.
	Verified bool `json:"verified" db:"verified"`
	// ImportedBy is the profile that pulled the item in, empty for platform-curated items.
	ImportedBy string    `json:"imported_by" db:"imported_by"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time `json:"updated_at" db:"updated_at"`
}

// Item origins.
const (
	OriginCurated = "curated"
	OriginImport  = "import"
)

// UserMetadata represents user-specific overrides or extensions of an item.
//
// Deprecated: retained as the wire/compat shape for the original /items/{id}/metadata
// endpoint. Overrides maps to ProfileItemLayer.PublicMetadata and OpenData maps to
// ProfileItemLayer.PrivateMetadata.
type UserMetadata struct {
	UserID         string    `json:"user_id" db:"user_id"`
	ItemID         string    `json:"item_id" db:"item_id"`
	CustomImageURL string    `json:"custom_image_url" db:"custom_image_url"`
	Overrides      Metadata  `json:"overrides" db:"overrides"` // Namespaced by SchemaID
	OpenData       Metadata  `json:"open_data" db:"open_data"` // The "OpenSchema" Wild West
	UpdatedAt      time.Time `json:"updated_at" db:"updated_at"`
}

// Metadata is a helper type for JSONB fields
type Metadata map[string]interface{}

func (m Metadata) Value() (driver.Value, error) {
	return json.Marshal(m)
}

func (m *Metadata) Scan(value interface{}) error {
	b, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("type assertion to []byte failed")
	}
	return json.Unmarshal(b, m)
}

// MergedItem is the final object served to the user after merging layers.
type MergedItem struct {
	ID       string                 `json:"id"`
	Name     string                 `json:"name"`
	ImageURL string                 `json:"image_url"`
	Sources  []ResolvedSource       `json:"sources"`
	Metadata map[string]interface{} `json:"metadata"`
}

// Supplier represents a retailer.
type Supplier struct {
	ID                string `json:"id" db:"id"`
	Name              string `json:"name" db:"name"`
	BaseURL           string `json:"base_url" db:"base_url"`
	AffiliateTemplate string `json:"affiliate_template" db:"affiliate_template"`
}

// ItemSource links an item to a supplier.
type ItemSource struct {
	ID         string `json:"id" db:"id"`
	ItemID     string `json:"item_id" db:"item_id"`
	SupplierID string `json:"supplier_id" db:"supplier_id"`
	ProductID  string `json:"product_id" db:"product_id"`
	// SourceURL is the canonical product URL the import came from. It is kept alongside
	// ProductID so an item can always be traced back to its origin page even if the
	// supplier's affiliate template changes.
	SourceURL   string    `json:"source_url" db:"source_url"`
	Price       float64   `json:"price" db:"price"`
	Currency    string    `json:"currency" db:"currency"`
	LastUpdated time.Time `json:"last_updated" db:"last_updated"`
}

// ResolvedSource is the final templated source for the UI.
type ResolvedSource struct {
	SupplierName string  `json:"supplier_name"`
	Price        float64 `json:"price"`
	URL          string  `json:"url"`
}

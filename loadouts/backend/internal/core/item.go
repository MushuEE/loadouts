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
type Item struct {
	ID           string    `json:"id" db:"id"`
	Name         string    `json:"name" db:"name"`
	ImageURL     string    `json:"image_url" db:"image_url"`
	BaseMetadata Metadata  `json:"base_metadata" db:"base_metadata"` // Namespaced by SchemaID
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}

// UserMetadata represents user-specific overrides or extensions of an item.
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
	ID          string    `json:"id" db:"id"`
	ItemID      string    `json:"item_id" db:"item_id"`
	SupplierID  string    `json:"supplier_id" db:"supplier_id"`
	ProductID   string    `json:"product_id" db:"product_id"`
	Price       float64   `json:"price" db:"price"`
	LastUpdated time.Time `json:"last_updated" db:"last_updated"`
}

// ResolvedSource is the final templated source for the UI.
type ResolvedSource struct {
	SupplierName string  `json:"supplier_name"`
	Price        float64 `json:"price"`
	URL          string  `json:"url"`
}

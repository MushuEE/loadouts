package core

import (
	"encoding/json"
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
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	BaseMetadata map[string]interface{} `json:"base_metadata"` // Namespaced by SchemaID
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

// UserMetadata represents user-specific overrides or extensions of an item.
type UserMetadata struct {
	UserID    string                 `json:"user_id"`
	ItemID    string                 `json:"item_id"`
	Overrides map[string]interface{} `json:"overrides"`   // Namespaced by SchemaID
	OpenData  map[string]interface{} `json:"open_data"`   // The "OpenSchema" Wild West
	UpdatedAt time.Time              `json:"updated_at"`
}

// MergedItem is the final object served to the user after merging layers.
type MergedItem struct {
	ID       string                 `json:"id"`
	Name     string                 `json:"name"`
	Metadata map[string]interface{} `json:"metadata"`
}

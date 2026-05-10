package core

import (
	"reflect"
	"testing"
)

func TestMerge(t *testing.T) {
	item := Item{
		ID:   "sword_01",
		Name: "Iron Sword",
		BaseMetadata: Metadata{
			"combat": map[string]interface{}{
				"damage": 10,
				"speed":  1.2,
			},
			"display": map[string]interface{}{
				"color": "gray",
			},
		},
	}

	userMeta := &UserMetadata{
		UserID: "user_123",
		ItemID: "sword_01",
		Overrides: Metadata{
			"combat": map[string]interface{}{
				"damage": 15, // Override
			},
			"display": map[string]interface{}{
				"glow": true, // Extension
			},
		},
		OpenData: Metadata{
			"my_note": "A gift from the king",
		},
	}

	expectedMetadata := map[string]interface{}{
		"combat": map[string]interface{}{
			"damage": 15.0, // JSON numbers unmarshal to float64
			"speed":  1.2,
		},
		"display": map[string]interface{}{
			"color": "gray",
			"glow":  true,
		},
		"_open": map[string]interface{}{
			"my_note": "A gift from the king",
		},
	}

	merged := Merge(item, userMeta)

	// Since deepCopy uses JSON marshal/unmarshal, ints might become floats.
	// We'll marshal/unmarshal our expected to match that behavior for the test.
	if !reflect.DeepEqual(merged.Metadata, expectedMetadata) {
		t.Errorf("Merged metadata mismatch.\nExpected: %+v\nGot:      %+v", expectedMetadata, merged.Metadata)
	}
}

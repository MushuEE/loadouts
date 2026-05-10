package core

import (
	"encoding/json"
)

// Merge performs a deep merge of base item metadata and user overrides.
// It also attaches the OpenData (OpenSchema) to the final result.
func Merge(item Item, userMeta *UserMetadata) MergedItem {
	mergedMetadata := make(map[string]interface{})
	finalImageURL := item.ImageURL

	// 1. Start with Base Metadata
	for k, v := range item.BaseMetadata {
		mergedMetadata[k] = deepCopy(v)
	}

	// 2. Apply Overrides (if any)
	if userMeta != nil {
		for k, v := range userMeta.Overrides {
			if existing, ok := mergedMetadata[k]; ok {
				// If both are maps, deep merge them
				if existingMap, ok := existing.(map[string]interface{}); ok {
					if overrideMap, ok := v.(map[string]interface{}); ok {
						mergedMetadata[k] = deepMergeMaps(existingMap, overrideMap)
						continue
					}
				}
			}
			// Otherwise, overwrite or add
			mergedMetadata[k] = deepCopy(v)
		}

		// 3. Attach OpenData under a reserved key
		if len(userMeta.OpenData) > 0 {
			mergedMetadata["_open"] = deepCopy(userMeta.OpenData)
		}

		// 4. Handle Image Override
		if userMeta.CustomImageURL != "" {
			finalImageURL = userMeta.CustomImageURL
		}
	}

	return MergedItem{
		ID:       item.ID,
		Name:     item.Name,
		ImageURL: finalImageURL,
		Metadata: mergedMetadata,
	}
}

func deepMergeMaps(base, override map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range base {
		result[k] = deepCopy(v)
	}
	for k, v := range override {
		if existing, ok := result[k]; ok {
			if existingMap, ok := existing.(map[string]interface{}); ok {
				if overrideMap, ok := v.(map[string]interface{}); ok {
					result[k] = deepMergeMaps(existingMap, overrideMap)
					continue
				}
			}
		}
		result[k] = deepCopy(v)
	}
	return result
}

func deepCopy(v interface{}) interface{} {
	// A simple but effective way to deep copy in Go without reflection-heavy libs
	// for this prototype. For production, a specialized deepcopy lib is better.
	b, _ := json.Marshal(v)
	var copy interface{}
	json.Unmarshal(b, &copy)
	return copy
}

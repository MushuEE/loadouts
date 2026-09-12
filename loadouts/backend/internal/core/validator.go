package core

import (
	"encoding/json"
	"fmt"

	"github.com/xeipuuv/gojsonschema"
)

// ValidateMetadata checks if the provided metadata conforms to the given schema definition.
func ValidateMetadata(schemaDef SchemaDefinition, metadata interface{}) error {
	schemaLoader := gojsonschema.NewGoLoader(schemaDef.Definition)
	documentLoader := gojsonschema.NewGoLoader(metadata)

	result, err := gojsonschema.Validate(schemaLoader, documentLoader)
	if err != nil {
		return fmt.Errorf("failed to execute validation: %w", err)
	}

	if !result.Valid() {
		var errMsgs string
		for _, desc := range result.Errors() {
			errMsgs += fmt.Sprintf("- %s\n", desc)
		}
		return fmt.Errorf("validation failed for schema %s (v%s):\n%s", schemaDef.ID, schemaDef.Version, errMsgs)
	}

	return nil
}

// ValidateSchemaDocument compiles a JSON Schema document without validating anything
// against it. Plugins ship schemas in their manifests, and a schema that does not compile
// would reject every metadata write it is supposed to gate, so it is checked at publish.
func ValidateSchemaDocument(raw json.RawMessage) error {
	if len(raw) == 0 {
		return fmt.Errorf("schema definition is empty")
	}
	var doc interface{}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return fmt.Errorf("schema definition is not valid JSON: %w", err)
	}
	if _, err := gojsonschema.NewSchema(gojsonschema.NewGoLoader(doc)); err != nil {
		return fmt.Errorf("schema definition is not a valid JSON Schema: %w", err)
	}
	return nil
}

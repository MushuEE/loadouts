package core

import (
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

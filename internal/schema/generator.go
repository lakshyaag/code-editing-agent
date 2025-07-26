package schema

import (
	"encoding/json"
	"fmt"

	"github.com/invopop/jsonschema"
)

// GenerateSchema generates a JSON schema for a given type
func GenerateSchema[T any]() map[string]interface{} {
	reflector := jsonschema.Reflector{
		AllowAdditionalProperties: true,
		DoNotReference:            true,
	}
	var v T

	schema := reflector.Reflect(v)

	// Marshal the schema to JSON
	schemaBytes, err := json.Marshal(schema)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal schema: %v", err))
	}

	var params map[string]interface{}
	if err := json.Unmarshal(schemaBytes, &params); err != nil {
		panic(fmt.Sprintf("failed to unmarshal schema to map: %v", err))
	}
	return params
}

// schema.go
package openapi2mcp

import (
	"fmt"
	"os"

	"github.com/getkin/kin-openapi/openapi3"
)

// extractProperty recursively extracts a property schema from an OpenAPI SchemaRef.
// Handles allOf, oneOf, anyOf, discriminator, default, example, and basic OpenAPI 3.1 features.
func extractProperty(s *openapi3.SchemaRef) map[string]any {
	if s == nil || s.Value == nil {
		return nil
	}
	val := s.Value
	prop := map[string]any{}
	// Handle allOf (merge all subschemas)
	if len(val.AllOf) > 0 {
		merged := map[string]any{}
		for _, sub := range val.AllOf {
			subProp := extractProperty(sub)
			for k, v := range subProp {
				merged[k] = v
			}
		}
		for k, v := range merged {
			prop[k] = v
		}
	}
	// Handle oneOf/anyOf (just include as-is for now)
	if len(val.OneOf) > 0 {
		fmt.Fprintf(os.Stderr, "[WARN] oneOf used in schema at %p. Only basic support is provided.\n", val)
		oneOf := []any{}
		for _, sub := range val.OneOf {
			oneOf = append(oneOf, extractProperty(sub))
		}
		prop["oneOf"] = oneOf
	}
	if len(val.AnyOf) > 0 {
		fmt.Fprintf(os.Stderr, "[WARN] anyOf used in schema at %p. Only basic support is provided.\n", val)
		anyOf := []any{}
		for _, sub := range val.AnyOf {
			anyOf = append(anyOf, extractProperty(sub))
		}
		prop["anyOf"] = anyOf
	}
	// Handle discriminator (OpenAPI 3.0/3.1)
	if val.Discriminator != nil {
		fmt.Fprintf(os.Stderr, "[WARN] discriminator used in schema at %p. Only basic support is provided.\n", val)
		prop["discriminator"] = val.Discriminator
	}
	// Type, description, enum, default, example
	if val.Type != "" {
		prop["type"] = val.Type
	}
	if val.Description != "" {
		prop["description"] = val.Description
	}
	if len(val.Enum) > 0 {
		prop["enum"] = val.Enum
	}
	if val.Default != nil {
		prop["default"] = val.Default
	}
	if val.Example != nil {
		prop["example"] = val.Example
	}
	// Object properties
	if val.Type == "object" && val.Properties != nil {
		objProps := map[string]any{}
		for name, sub := range val.Properties {
			objProps[name] = extractProperty(sub)
		}
		prop["properties"] = objProps
		if len(val.Required) > 0 {
			prop["required"] = val.Required
		}
	}
	// Array items
	if val.Type == "array" && val.Items != nil {
		prop["items"] = extractProperty(val.Items)
	}
	return prop
}

// BuildInputSchema converts OpenAPI parameters and request body schema to a single JSON Schema object for MCP tool input validation.
// Returns a JSON Schema as a map[string]any.
// Example usage for BuildInputSchema:
//
//	params := ... // openapi3.Parameters from an operation
//	reqBody := ... // *openapi3.RequestBodyRef from an operation
//	schema := openapi2mcp.BuildInputSchema(params, reqBody)
//	// schema is a map[string]any representing the JSON schema for tool input
func BuildInputSchema(params openapi3.Parameters, requestBody *openapi3.RequestBodyRef) map[string]any {
	schema := map[string]any{
		"type":       "object",
		"properties": map[string]any{},
	}
	properties := schema["properties"].(map[string]any)
	var required []string

	// Parameters (query, path, header, cookie)
	for _, paramRef := range params {
		if paramRef == nil || paramRef.Value == nil {
			continue
		}
		p := paramRef.Value
		if p.Schema != nil && p.Schema.Value != nil {
			if p.Schema.Value.Type == "string" && p.Schema.Value.Format == "binary" {
				fmt.Fprintf(os.Stderr, "[WARN] Parameter '%s' uses 'string' with 'binary' format. Non-JSON body types are not fully supported.\n", p.Name)
			}
			prop := extractProperty(p.Schema)
			if p.Description != "" {
				prop["description"] = p.Description
			}
			properties[p.Name] = prop
			if p.Required {
				required = append(required, p.Name)
			}
		}
		// Warn about unsupported parameter locations
		if p.In != "query" && p.In != "path" && p.In != "header" && p.In != "cookie" {
			fmt.Fprintf(os.Stderr, "[WARN] Parameter '%s' uses unsupported location '%s'.\n", p.Name, p.In)
		}
	}

	// Request body (only application/json for now)
	if requestBody != nil && requestBody.Value != nil {
		for mtName := range requestBody.Value.Content {
			if mtName != "application/json" {
				fmt.Fprintf(os.Stderr, "[WARN] Request body uses media type '%s'. Only 'application/json' is fully supported.\n", mtName)
			}
		}
		if mt := requestBody.Value.Content.Get("application/json"); mt != nil && mt.Schema != nil && mt.Schema.Value != nil {
			bodyProp := extractProperty(mt.Schema)
			bodyProp["description"] = "The JSON request body."
			properties["requestBody"] = bodyProp
			if requestBody.Value.Required {
				required = append(required, "requestBody")
			}
		}
	}

	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

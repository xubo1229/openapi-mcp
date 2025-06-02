// schema.go
package openapi2mcp

import (
	"fmt"
	"os"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

// escapeParameterName converts parameter names with brackets to MCP-compatible names.
// For example: "filter[created_at]" becomes "filter_created_at_"
// The trailing underscore distinguishes escaped names from naturally occurring names.
func escapeParameterName(name string) string {
	if !strings.Contains(name, "[") && !strings.Contains(name, "]") {
		return name // No escaping needed
	}

	// Replace brackets with underscores and add trailing underscore
	escaped := strings.ReplaceAll(name, "[", "_")
	escaped = strings.ReplaceAll(escaped, "]", "_")

	// Add trailing underscore if not already present to mark as escaped
	if !strings.HasSuffix(escaped, "_") {
		escaped += "_"
	}

	return escaped
}

// unescapeParameterName converts escaped parameter names back to their original form.
// This maintains a mapping from escaped names to original names for parameter lookup.
func unescapeParameterName(escaped string, originalNames map[string]string) string {
	if original, exists := originalNames[escaped]; exists {
		return original
	}
	return escaped // Return as-is if not found in mapping
}

// buildParameterNameMapping creates a mapping from escaped parameter names to original names.
// This is used to reverse the escaping when looking up parameter values.
func buildParameterNameMapping(params openapi3.Parameters) map[string]string {
	mapping := make(map[string]string)
	for _, paramRef := range params {
		if paramRef == nil || paramRef.Value == nil {
			continue
		}
		p := paramRef.Value
		escaped := escapeParameterName(p.Name)
		if escaped != p.Name {
			mapping[escaped] = p.Name
		}
	}
	return mapping
}

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
	// Type, format, description, enum, default, example
	if val.Type != nil && len(*val.Type) > 0 {
		// Use the first type if multiple types are specified
		prop["type"] = (*val.Type)[0]
	}
	if val.Format != "" {
		prop["format"] = val.Format
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
	if val.Type != nil && val.Type.Is("object") && val.Properties != nil {
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
	if val.Type != nil && val.Type.Is("array") && val.Items != nil {
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
			if p.Schema.Value.Type != nil && p.Schema.Value.Type.Is("string") && p.Schema.Value.Format == "binary" {
				fmt.Fprintf(os.Stderr, "[WARN] Parameter '%s' uses 'string' with 'binary' format. Non-JSON body types are not fully supported.\n", p.Name)
			}
			prop := extractProperty(p.Schema)
			if p.Description != "" {
				prop["description"] = p.Description
			}
			// Use escaped parameter name for MCP schema compatibility
			escapedName := escapeParameterName(p.Name)
			properties[escapedName] = prop
			if p.Required {
				required = append(required, escapedName)
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

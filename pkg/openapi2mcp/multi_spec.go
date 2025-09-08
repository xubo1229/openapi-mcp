package openapi2mcp

import (
	"fmt"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

// LoadMultipleOpenAPISpecsFromString loads and validates multiple OpenAPI specs from a single string.
// Specs should be separated by YAML document separators (---).
// Returns a slice of parsed OpenAPI documents or an error if any spec fails to load.
func LoadMultipleOpenAPISpecsFromString(data string) ([]*openapi3.T, error) {
	// Split by YAML document separator
	data = strings.ReplaceAll(data, "\r\n", "\n")
	specs := strings.Split(data, "\n---\n")

	// Filter out empty specs
	var validSpecs []string
	for _, spec := range specs {
		trimmed := strings.TrimSpace(spec)
		if trimmed != "" {
			validSpecs = append(validSpecs, trimmed)
		}
	}

	if len(validSpecs) == 0 {
		return nil, fmt.Errorf("no valid OpenAPI specs found in input")
	}

	var docs []*openapi3.T
	var errors []error

	for i, spec := range validSpecs {
		doc, err := LoadOpenAPISpecFromBytes([]byte(spec))
		if err != nil {
			errors = append(errors, fmt.Errorf("spec #%d failed: %v", i+1, err))
			continue
		}
		docs = append(docs, doc)
	}

	if len(errors) > 0 {
		// If all specs failed, return the first error
		if len(errors) == len(validSpecs) {
			return nil, fmt.Errorf("all %d specs failed to load: %v", len(validSpecs), errors[0])
		}
		// If some specs failed, return the successful ones but log warnings
		// The caller can decide whether to treat this as an error
		return docs, fmt.Errorf("some specs failed to load: %v", errors)
	}

	return docs, nil
}

// MergeOpenAPISpecs merges multiple OpenAPI specs into a single spec.
// This is a simplified merge that combines paths, but doesn't handle all edge cases.
// For production use, a more sophisticated merging strategy may be needed.
func MergeOpenAPISpecs(docs []*openapi3.T) (*openapi3.T, error) {
	if len(docs) == 0 {
		return nil, fmt.Errorf("no specs to merge")
	}

	if len(docs) == 1 {
		return docs[0], nil
	}

	// Use the first spec as the base
	merged := docs[0]

	// Merge paths from other specs
	for i := 1; i < len(docs); i++ {
		doc := docs[i]

		// Merge paths
		if doc.Paths != nil {
			for path, pathItem := range doc.Paths.Map() {
				if merged.Paths != nil {
					// Check if path already exists
					if existing := merged.Paths.Find(path); existing == nil {
						merged.Paths.Set(path, pathItem)
					}
					// Note: This simplistic approach doesn't handle path conflicts properly
				}
			}
		}

		// Merge components (schemas, parameters, etc.)
		if doc.Components != nil {
			if merged.Components == nil {
				merged.Components = &openapi3.Components{}
			}

			// Merge schemas
			if doc.Components.Schemas != nil {
				if merged.Components.Schemas == nil {
					merged.Components.Schemas = make(map[string]*openapi3.SchemaRef)
				}
				for name, schema := range doc.Components.Schemas {
					if _, exists := merged.Components.Schemas[name]; !exists {
						merged.Components.Schemas[name] = schema
					}
					// Note: This simplistic approach doesn't handle schema name conflicts properly
				}
			}
		}
	}

	return merged, nil
}

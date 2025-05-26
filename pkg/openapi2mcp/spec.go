// spec.go
package openapi2mcp

import (
	"fmt"
	"os"
	"regexp"

	"github.com/getkin/kin-openapi/openapi3"
)

// LoadOpenAPISpec loads and parses an OpenAPI YAML or JSON file from the given path.
// Returns the parsed OpenAPI document or an error.
// Example usage for LoadOpenAPISpec:
//
//	doc, err := openapi2mcp.LoadOpenAPISpec("petstore.yaml")
//	if err != nil { log.Fatal(err) }
//	ops := openapi2mcp.ExtractOpenAPIOperations(doc)
func LoadOpenAPISpec(path string) (*openapi3.T, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read OpenAPI spec file: %w", err)
	}
	return LoadOpenAPISpecFromBytes(data)
}

// LoadOpenAPISpecFromString loads and parses an OpenAPI YAML or JSON spec from a string.
// Returns the parsed OpenAPI document or an error.
func LoadOpenAPISpecFromString(data string) (*openapi3.T, error) {
	return LoadOpenAPISpecFromBytes([]byte(data))
}

// LoadOpenAPISpecFromBytes loads and parses an OpenAPI YAML or JSON spec from a byte slice.
// Returns the parsed OpenAPI document or an error.
func LoadOpenAPISpecFromBytes(data []byte) (*openapi3.T, error) {
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromData(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse OpenAPI spec: %w", err)
	}
	if err := doc.Validate(loader.Context); err != nil {
		return nil, fmt.Errorf("OpenAPI spec validation failed: %w", err)
	}
	return doc, nil
}

// ExtractOpenAPIOperations extracts all operations from the OpenAPI spec, merging path-level and operation-level parameters.
// Returns a slice of OpenAPIOperation describing each operation.
// Example usage for ExtractOpenAPIOperations:
//
//	doc, err := openapi2mcp.LoadOpenAPISpec("petstore.yaml")
//	if err != nil { log.Fatal(err) }
//	ops := openapi2mcp.ExtractOpenAPIOperations(doc)
func ExtractOpenAPIOperations(doc *openapi3.T) []OpenAPIOperation {
	var ops []OpenAPIOperation
	for path, pathItem := range doc.Paths {
		for method, op := range pathItem.Operations() {
			id := op.OperationID
			if id == "" {
				id = fmt.Sprintf("%s_%s", method, path)
			}
			desc := op.Description

			// Merge path-level and operation-level parameters
			mergedParams := openapi3.Parameters{}
			if pathItem.Parameters != nil {
				mergedParams = append(mergedParams, pathItem.Parameters...)
			}
			if op.Parameters != nil {
				mergedParams = append(mergedParams, op.Parameters...)
			}

			tags := op.Tags
			var security openapi3.SecurityRequirements
			if op.Security != nil {
				security = *op.Security
			} else {
				security = doc.Security
			}
			ops = append(ops, OpenAPIOperation{
				OperationID: id,
				Summary:     op.Summary,
				Description: desc,
				Path:        path,
				Method:      method,
				Parameters:  mergedParams,
				RequestBody: op.RequestBody,
				Tags:        tags,
				Security:    security,
			})
		}
	}
	return ops
}

// ExtractFilteredOpenAPIOperations returns only those operations whose description matches includeRegex (if not nil) and does not match excludeRegex (if not nil).
// Returns a filtered slice of OpenAPIOperation.
// Example usage for ExtractFilteredOpenAPIOperations:
//
//	include := regexp.MustCompile("pets")
//	filtered := openapi2mcp.ExtractFilteredOpenAPIOperations(doc, include, nil)
func ExtractFilteredOpenAPIOperations(doc *openapi3.T, includeRegex, excludeRegex *regexp.Regexp) []OpenAPIOperation {
	all := ExtractOpenAPIOperations(doc)
	var filtered []OpenAPIOperation
	for _, op := range all {
		desc := op.Description
		if desc == "" {
			desc = op.Summary
		}
		if includeRegex != nil && !includeRegex.MatchString(desc) {
			continue
		}
		if excludeRegex != nil && excludeRegex.MatchString(desc) {
			continue
		}
		filtered = append(filtered, op)
	}
	return filtered
}

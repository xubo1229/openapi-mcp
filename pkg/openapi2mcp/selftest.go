// selftest.go
package openapi2mcp

import (
	"fmt"
	"os"
	"regexp"

	"github.com/getkin/kin-openapi/openapi3"
)

// SelfTestOpenAPIMCP checks that the generated MCP server matches the OpenAPI contract (basic: all required tools and arguments are present).
// Returns an error if any required tools or arguments are missing.
func SelfTestOpenAPIMCP(doc *openapi3.T, toolNames []string) error {
	ops := ExtractOpenAPIOperations(doc)
	failures := 0
	warnings := 0
	toolMap := map[string]struct{}{}
	for _, name := range toolNames {
		toolMap[name] = struct{}{}
	}
	recommendedTypes := map[string]bool{"string": true, "integer": true, "boolean": true, "number": true, "array": true, "object": true}
	recommendedLocations := map[string]bool{"path": true, "query": true, "header": true, "cookie": true}

	// Check for missing operationIds in the original spec
	for path, pathItem := range doc.Paths {
		for method, operation := range pathItem.Operations() {
			if operation.OperationID == "" {
				fmt.Fprintf(os.Stderr, "[ERROR] Operation for path '%s' and method '%s' is missing an operationId.\n", path, method)
				fmt.Fprintf(os.Stderr, "  Suggestion: Add an 'operationId' field, e.g.\n    %s:\n      %s:\n        operationId: <uniqueOperationId>\n", path, method)
				failures++
			}
		}
	}

	for _, op := range ops {
		if _, ok := toolMap[op.OperationID]; !ok && op.OperationID != "" {
			fmt.Fprintf(os.Stderr, "[ERROR] Tool '%s' (operationId) is missing from MCP server.\n", op.OperationID)
			fmt.Fprintf(os.Stderr, "  Suggestion: Ensure the operationId '%s' is unique and present in the OpenAPI spec.\n", op.OperationID)
			failures++
		}
		if op.Summary == "" {
			fmt.Fprintf(os.Stderr, "[WARN] Operation '%s' (path: '%s', method: '%s') is missing a summary.\n", op.OperationID, op.Path, op.Method)
			fmt.Fprintf(os.Stderr, "  Suggestion: Add a 'summary' field to describe the operation's purpose.\n")
			warnings++
		}
		if op.Description == "" {
			fmt.Fprintf(os.Stderr, "[WARN] Operation '%s' (path: '%s', method: '%s') is missing a description.\n", op.OperationID, op.Path, op.Method)
			fmt.Fprintf(os.Stderr, "  Suggestion: Add a 'description' field for more detail.\n")
			warnings++
		}
		if len(op.Tags) == 0 {
			fmt.Fprintf(os.Stderr, "[WARN] Operation '%s' (path: '%s', method: '%s') has no tags.\n", op.OperationID, op.Path, op.Method)
			fmt.Fprintf(os.Stderr, "  Suggestion: Add tags to group related operations.\n")
			warnings++
		}
		// Parameter checks
		for _, paramRef := range op.Parameters {
			if paramRef == nil || paramRef.Value == nil {
				continue
			}
			p := paramRef.Value
			if p.Name == "" {
				fmt.Fprintf(os.Stderr, "[ERROR] Operation '%s' has a parameter with no name.\n", op.OperationID)
				fmt.Fprintf(os.Stderr, "  Suggestion: Add a 'name' field to the parameter.\n")
				failures++
			}
			if !recommendedLocations[p.In] {
				fmt.Fprintf(os.Stderr, "[WARN] Parameter '%s' in operation '%s' uses non-standard location '%s'.\n", p.Name, op.OperationID, p.In)
				fmt.Fprintf(os.Stderr, "  Suggestion: Use one of: path, query, header, cookie.\n")
				warnings++
			}
			if p.Schema == nil || p.Schema.Value == nil {
				fmt.Fprintf(os.Stderr, "[ERROR] Parameter '%s' in operation '%s' is missing a schema/type.\n", p.Name, op.OperationID)
				fmt.Fprintf(os.Stderr, "  Suggestion: Add a 'schema' with a 'type', e.g.\n    - name: %s\n      in: %s\n      schema:\n        type: string\n", p.Name, p.In)
				failures++
				continue
			}
			schema := p.Schema.Value
			typeStr := schema.Type
			if typeStr == "" {
				fmt.Fprintf(os.Stderr, "[ERROR] Parameter '%s' in operation '%s' is missing a type in its schema.\n", p.Name, op.OperationID)
				fmt.Fprintf(os.Stderr, "  Suggestion: Add a 'type' to the schema, e.g. type: string\n")
				failures++
			} else if !recommendedTypes[typeStr] {
				fmt.Fprintf(os.Stderr, "[WARN] Parameter '%s' in operation '%s' uses uncommon type '%s'.\n", p.Name, op.OperationID, typeStr)
				fmt.Fprintf(os.Stderr, "  Suggestion: Use one of: string, integer, boolean, number, array, object.\n")
				warnings++
			}
			// Enum/default/example suggestions
			if typeStr == "string" || typeStr == "integer" || typeStr == "boolean" {
				if len(schema.Enum) == 0 {
					fmt.Fprintf(os.Stderr, "[INFO] Parameter '%s' in operation '%s' has no enum.\n", p.Name, op.OperationID)
					fmt.Fprintf(os.Stderr, "  Suggestion: Add an 'enum' if the parameter has a fixed set of values.\n")
					warnings++
				}
				if schema.Default == nil {
					fmt.Fprintf(os.Stderr, "[INFO] Parameter '%s' in operation '%s' has no default value.\n", p.Name, op.OperationID)
					fmt.Fprintf(os.Stderr, "  Suggestion: Add a 'default' value for better UX.\n")
					warnings++
				}
				if schema.Example == nil {
					fmt.Fprintf(os.Stderr, "[INFO] Parameter '%s' in operation '%s' has no example.\n", p.Name, op.OperationID)
					fmt.Fprintf(os.Stderr, "  Suggestion: Add an 'example' for documentation and testing.\n")
					warnings++
				}
				// Enum/default consistency
				if len(schema.Enum) > 0 && schema.Default != nil {
					found := false
					for _, v := range schema.Enum {
						if v == schema.Default {
							found = true
							break
						}
					}
					if !found {
						fmt.Fprintf(os.Stderr, "[WARN] Parameter '%s' in operation '%s' has a default value not in its enum list.\n", p.Name, op.OperationID)
						fmt.Fprintf(os.Stderr, "  Suggestion: Ensure the default value is one of the enum values.\n")
						warnings++
					}
				}
			}
		}
		// Request body checks
		if op.RequestBody != nil && op.RequestBody.Value != nil {
			for mtName, mt := range op.RequestBody.Value.Content {
				if mt.Schema == nil || mt.Schema.Value == nil {
					fmt.Fprintf(os.Stderr, "[ERROR] Request body for operation '%s' (media type: '%s') is missing a schema/type.\n", op.OperationID, mtName)
					fmt.Fprintf(os.Stderr, "  Suggestion: Add a 'schema' with a 'type', e.g. type: object\n")
					failures++
					continue
				}
				schema := mt.Schema.Value
				typeStr := schema.Type
				if typeStr == "" {
					fmt.Fprintf(os.Stderr, "[ERROR] Request body for operation '%s' (media type: '%s') is missing a type in its schema.\n", op.OperationID, mtName)
					fmt.Fprintf(os.Stderr, "  Suggestion: Add a 'type' to the schema, e.g. type: object\n")
					failures++
				} else if !recommendedTypes[typeStr] {
					fmt.Fprintf(os.Stderr, "[WARN] Request body for operation '%s' uses uncommon type '%s'.\n", op.OperationID, typeStr)
					fmt.Fprintf(os.Stderr, "  Suggestion: Use one of: string, integer, boolean, number, array, object.\n")
					warnings++
				}
				// Enum/default/example suggestions for request body properties
				if typeStr == "object" && schema.Properties != nil {
					for propName, propRef := range schema.Properties {
						if propRef == nil || propRef.Value == nil {
							continue
						}
						prop := propRef.Value
						ptype := prop.Type
						if ptype == "string" || ptype == "integer" || ptype == "boolean" {
							if len(prop.Enum) == 0 {
								fmt.Fprintf(os.Stderr, "[INFO] Request body property '%s' in operation '%s' has no enum.\n", propName, op.OperationID)
								fmt.Fprintf(os.Stderr, "  Suggestion: Add an 'enum' if the property has a fixed set of values.\n")
								warnings++
							}
							if prop.Default == nil {
								fmt.Fprintf(os.Stderr, "[INFO] Request body property '%s' in operation '%s' has no default value.\n", propName, op.OperationID)
								fmt.Fprintf(os.Stderr, "  Suggestion: Add a 'default' value for better UX.\n")
								warnings++
							}
							if prop.Example == nil {
								fmt.Fprintf(os.Stderr, "[INFO] Request body property '%s' in operation '%s' has no example.\n", propName, op.OperationID)
								fmt.Fprintf(os.Stderr, "  Suggestion: Add an 'example' for documentation and testing.\n")
								warnings++
							}
							if len(prop.Enum) > 0 && prop.Default != nil {
								found := false
								for _, v := range prop.Enum {
									if v == prop.Default {
										found = true
										break
									}
								}
								if !found {
									fmt.Fprintf(os.Stderr, "[WARN] Request body property '%s' in operation '%s' has a default value not in its enum list.\n", propName, op.OperationID)
									fmt.Fprintf(os.Stderr, "  Suggestion: Ensure the default value is one of the enum values.\n")
									warnings++
								}
							}
						}
					}
				}
			}
		}
		// Schema required argument checks
		inputSchema := BuildInputSchema(op.Parameters, op.RequestBody)
		props, _ := inputSchema["properties"].(map[string]any)
		if reqList, ok := inputSchema["required"].([]string); ok {
			for _, req := range reqList {
				if _, ok := props[req]; !ok {
					fmt.Fprintf(os.Stderr, "[ERROR] Tool '%s' is missing required argument '%s' in schema.\n", op.OperationID, req)
					// Try to suggest the type if possible
					typeHint := "string"
					if param, found := findParamByName(op.Parameters, req); found && param.Schema != nil && param.Schema.Value != nil && param.Schema.Value.Type != "" {
						typeHint = param.Schema.Value.Type
					}
					fmt.Fprintf(os.Stderr, "  Suggestion: Add the required argument '%s' (type: %s) to the schema for tool '%s' (path: '%s', method: '%s').\n", req, typeHint, op.OperationID, op.Path, op.Method)
					fmt.Fprintf(os.Stderr, "    Example property: %s: { type: %q }\n", req, typeHint)
					failures++
				}
				// Warn if required field is missing an example
				if param, found := findParamByName(op.Parameters, req); found && param.Schema != nil && param.Schema.Value != nil && (param.Schema.Value.Example == nil) {
					fmt.Fprintf(os.Stderr, "[INFO] Required parameter '%s' in operation '%s' has no example.\n", req, op.OperationID)
					fmt.Fprintf(os.Stderr, "  Suggestion: Add an 'example' for this required parameter.\n")
					warnings++
				}
			}
		}
		// Cross-field consistency: check if required parameters are mentioned in summary or description
		if reqList, ok := inputSchema["required"].([]string); ok {
			for _, req := range reqList {
				mentioned := false
				if op.Summary != "" && containsWord(op.Summary, req) {
					mentioned = true
				}
				if op.Description != "" && containsWord(op.Description, req) {
					mentioned = true
				}
				if !mentioned {
					fmt.Fprintf(os.Stderr, "[INFO] Required parameter '%s' in operation '%s' is not mentioned in summary or description.\n", req, op.OperationID)
					fmt.Fprintf(os.Stderr, "  Suggestion: Document required parameters in the summary or description for clarity.\n")
					warnings++
				}
			}
		}
	}
	if failures > 0 || warnings > 0 {
		fmt.Fprintf(os.Stderr, "[INFO] See the suggestions above to fix the reported issues.\n")
	}
	if failures > 0 {
		return fmt.Errorf("self-test failed: %d errors, %d warnings. See errors and suggestions above.", failures, warnings)
	}
	if warnings > 0 {
		fmt.Fprintf(os.Stderr, "[INFO] Self-test passed with %d warnings.\n", warnings)
	} else {
		fmt.Fprintf(os.Stderr, "[INFO] Self-test passed: all tools and required arguments are present.\n")
	}
	return nil
}

// findParamByName returns the parameter with the given name, if present.
func findParamByName(params openapi3.Parameters, name string) (*openapi3.Parameter, bool) {
	for _, paramRef := range params {
		if paramRef != nil && paramRef.Value != nil && paramRef.Value.Name == name {
			return paramRef.Value, true
		}
	}
	return nil, false
}

// containsWord checks if a word is present in a string (case-insensitive, word boundary).
func containsWord(s, word string) bool {
	if len(word) == 0 || len(s) == 0 {
		return false
	}
	return regexp.MustCompile(`(?i)\\b` + regexp.QuoteMeta(word) + `\\b`).MatchString(s)
}

// SelfTestOpenAPIMCPWithOptions runs the self-test with or without detailed suggestions.
func SelfTestOpenAPIMCPWithOptions(doc *openapi3.T, toolNames []string, detailedSuggestions bool) error {
	if detailedSuggestions {
		return SelfTestOpenAPIMCP(doc, toolNames)
	}
	// Basic actionable errors only
	ops := ExtractOpenAPIOperations(doc)
	failures := 0
	toolMap := map[string]struct{}{}
	for _, name := range toolNames {
		toolMap[name] = struct{}{}
	}

	// Check for missing operationIds in the original spec
	for path, pathItem := range doc.Paths {
		for method, operation := range pathItem.Operations() {
			if operation.OperationID == "" {
				fmt.Fprintf(os.Stderr, "[ERROR] Operation for path '%s' and method '%s' is missing an operationId.\n", path, method)
				fmt.Fprintf(os.Stderr, "  Suggestion: Add an 'operationId' field, e.g.\n    %s:\n      %s:\n        operationId: <uniqueOperationId>\n", path, method)
				failures++
			}
		}
	}

	for _, op := range ops {
		if _, ok := toolMap[op.OperationID]; !ok && op.OperationID != "" {
			fmt.Fprintf(os.Stderr, "[ERROR] Tool '%s' (operationId) is missing from MCP server.\n", op.OperationID)
			fmt.Fprintf(os.Stderr, "  Suggestion: Ensure the operationId '%s' is unique and present in the OpenAPI spec.\n", op.OperationID)
			failures++
		}
		for _, paramRef := range op.Parameters {
			if paramRef == nil || paramRef.Value == nil {
				continue
			}
			p := paramRef.Value
			if p.Name == "" {
				fmt.Fprintf(os.Stderr, "[ERROR] Operation '%s' has a parameter with no name.\n", op.OperationID)
				fmt.Fprintf(os.Stderr, "  Suggestion: Add a 'name' field to the parameter.\n")
				failures++
			}
			if p.Schema == nil || p.Schema.Value == nil {
				fmt.Fprintf(os.Stderr, "[ERROR] Parameter '%s' in operation '%s' is missing a schema/type.\n", p.Name, op.OperationID)
				fmt.Fprintf(os.Stderr, "  Suggestion: Add a 'schema' with a 'type', e.g.\n    - name: %s\n      in: %s\n      schema:\n        type: string\n", p.Name, p.In)
				failures++
				continue
			}
			typeStr := p.Schema.Value.Type
			if typeStr == "" {
				fmt.Fprintf(os.Stderr, "[ERROR] Parameter '%s' in operation '%s' is missing a type in its schema.\n", p.Name, op.OperationID)
				fmt.Fprintf(os.Stderr, "  Suggestion: Add a 'type' to the schema, e.g. type: string\n")
				failures++
			}
		}
		// Request body checks
		if op.RequestBody != nil && op.RequestBody.Value != nil {
			for mtName, mt := range op.RequestBody.Value.Content {
				if mt.Schema == nil || mt.Schema.Value == nil {
					fmt.Fprintf(os.Stderr, "[ERROR] Request body for operation '%s' (media type: '%s') is missing a schema/type.\n", op.OperationID, mtName)
					fmt.Fprintf(os.Stderr, "  Suggestion: Add a 'schema' with a 'type', e.g. type: object\n")
					failures++
					continue
				}
				typeStr := mt.Schema.Value.Type
				if typeStr == "" {
					fmt.Fprintf(os.Stderr, "[ERROR] Request body for operation '%s' (media type: '%s') is missing a type in its schema.\n", op.OperationID, mtName)
					fmt.Fprintf(os.Stderr, "  Suggestion: Add a 'type' to the schema, e.g. type: object\n")
					failures++
				}
			}
		}
		// Schema required argument checks
		inputSchema := BuildInputSchema(op.Parameters, op.RequestBody)
		props, _ := inputSchema["properties"].(map[string]any)
		if reqList, ok := inputSchema["required"].([]string); ok {
			for _, req := range reqList {
				if _, ok := props[req]; !ok {
					fmt.Fprintf(os.Stderr, "[ERROR] Tool '%s' is missing required argument '%s' in schema.\n", op.OperationID, req)
					fmt.Fprintf(os.Stderr, "  Suggestion: Add the required argument '%s' to the schema for tool '%s' (path: '%s', method: '%s').\n", req, op.OperationID, op.Path, op.Method)
					fmt.Fprintf(os.Stderr, "    Example property: %s: { type: \"string\" }\n", req)
					failures++
				}
			}
		}
	}
	if failures > 0 {
		fmt.Fprintf(os.Stderr, "[INFO] See the suggestions above to fix the reported issues.\n")
		return fmt.Errorf("self-test failed: %d issues found. See errors and suggestions above.", failures)
	}
	fmt.Fprintf(os.Stderr, "[INFO] Self-test passed: all tools and required arguments are present.\n")
	return nil
}

// Example usage for SelfTestOpenAPIMCP:
//
//   doc, _ := openapi2mcp.LoadOpenAPISpec("petstore.yaml")
//   ops := openapi2mcp.ExtractOpenAPIOperations(doc)
//   srv := openapi2mcp.NewServer("petstore", doc.Info.Version, doc)
//   toolNames := srv.ListToolNames()
//   if err := openapi2mcp.SelfTestOpenAPIMCP(doc, toolNames); err != nil {
//       log.Fatal(err)
//   }

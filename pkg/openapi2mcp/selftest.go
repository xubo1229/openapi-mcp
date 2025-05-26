// selftest.go
package openapi2mcp

import (
	"fmt"
	"os"

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

	for _, op := range ops {
		if op.OperationID == "" {
			fmt.Fprintf(os.Stderr, "[ERROR] Operation for path '%s' and method '%s' is missing an operationId.\n", op.Path, op.Method)
			fmt.Fprintf(os.Stderr, "  Suggestion: Add an 'operationId' field, e.g.\n    %s:\n      %s:\n        operationId: <uniqueOperationId>\n", op.Path, op.Method)
			failures++
		}
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
				fmt.Fprintf(os.Stderr, "  Suggestion: Add a 'schema' with a 'type', e.g.\n    - name: %s\n      in: %s\n      schema:\n        type: string\n", p.Name, p.In, p.Name, p.In)
				failures++
				continue
			}
			typeStr := p.Schema.Value.Type
			if typeStr == "" {
				fmt.Fprintf(os.Stderr, "[ERROR] Parameter '%s' in operation '%s' is missing a type in its schema.\n", p.Name, op.OperationID)
				fmt.Fprintf(os.Stderr, "  Suggestion: Add a 'type' to the schema, e.g. type: string\n")
				failures++
			} else if !recommendedTypes[typeStr] {
				fmt.Fprintf(os.Stderr, "[WARN] Parameter '%s' in operation '%s' uses uncommon type '%s'.\n", p.Name, op.OperationID, typeStr)
				fmt.Fprintf(os.Stderr, "  Suggestion: Use one of: string, integer, boolean, number, array, object.\n")
				warnings++
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
				} else if !recommendedTypes[typeStr] {
					fmt.Fprintf(os.Stderr, "[WARN] Request body for operation '%s' uses uncommon type '%s'.\n", op.OperationID, typeStr)
					fmt.Fprintf(os.Stderr, "  Suggestion: Use one of: string, integer, boolean, number, array, object.\n")
					warnings++
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

// Example usage for SelfTestOpenAPIMCP:
//
//   doc, _ := openapi2mcp.LoadOpenAPISpec("petstore.yaml")
//   ops := openapi2mcp.ExtractOpenAPIOperations(doc)
//   srv := openapi2mcp.NewServer("petstore", doc.Info.Version, doc)
//   toolNames := srv.ListToolNames()
//   if err := openapi2mcp.SelfTestOpenAPIMCP(doc, toolNames); err != nil {
//       log.Fatal(err)
//   }

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
	toolMap := map[string]struct{}{}
	for _, name := range toolNames {
		toolMap[name] = struct{}{}
	}
	for _, op := range ops {
		if _, ok := toolMap[op.OperationID]; !ok {
			fmt.Fprintf(os.Stderr, "[ERROR] Tool '%s' (operationId) is missing from MCP server.\n", op.OperationID)
			failures++
		}
		inputSchema := BuildInputSchema(op.Parameters, op.RequestBody)
		props, _ := inputSchema["properties"].(map[string]any)
		if reqList, ok := inputSchema["required"].([]string); ok {
			for _, req := range reqList {
				if _, ok := props[req]; !ok {
					fmt.Fprintf(os.Stderr, "[ERROR] Tool '%s' is missing required argument '%s' in schema.\n", op.OperationID, req)
					failures++
				}
			}
		}
	}
	if failures > 0 {
		return fmt.Errorf("self-test failed: %d issues found. See errors above.", failures)
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

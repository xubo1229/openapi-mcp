// server.go
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/jedisct1/openapi-mcp/pkg/openapi2mcp"
)

// startServer starts the MCP server in stdio or HTTP mode, based on CLI flags.
// It registers all OpenAPI operations as MCP tools and starts the server.
func startServer(flags *cliFlags, ops []openapi2mcp.OpenAPIOperation, doc *openapi3.T) {
	srv := openapi2mcp.NewServerWithOps("openapi-mcp", doc.Info.Version, doc, ops)
	fmt.Fprintln(os.Stderr, "Registered all OpenAPI operations as MCP tools.")

	if flags.httpAddr != "" {
		fmt.Fprintf(os.Stderr, "Starting MCP server (HTTP) on %s (base path: %s)...\n", flags.httpAddr, flags.basePath)
		if err := openapi2mcp.ServeHTTP(srv, flags.httpAddr, flags.basePath); err != nil {
			log.Fatalf("Failed to start MCP HTTP server: %v", err)
		}
	} else {
		fmt.Fprintln(os.Stderr, "Starting MCP server (stdio)...")
		if err := openapi2mcp.ServeStdio(srv); err != nil {
			log.Fatalf("Failed to start MCP server: %v", err)
		}
	}
}

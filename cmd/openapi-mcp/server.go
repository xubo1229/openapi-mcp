// server.go
package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/getkin/kin-openapi/openapi3"
	mcpserver "github.com/jedisct1/openapi-mcp/pkg/mcp/server"
	"github.com/jedisct1/openapi-mcp/pkg/openapi2mcp"
)

// startServer starts the MCP server in stdio or HTTP mode, based on CLI flags.
// It registers all OpenAPI operations as MCP tools and starts the server.
func startServer(flags *cliFlags, ops []openapi2mcp.OpenAPIOperation, doc *openapi3.T) {
	if flags.httpAddr != "" && len(flags.mounts) > 0 {
		// Check for duplicate base paths
		basePathCount := make(map[string]int)
		for _, m := range flags.mounts {
			basePathCount[m.BasePath]++
		}
		var dups []string
		for base, count := range basePathCount {
			if count > 1 {
				dups = append(dups, base)
			}
		}
		if len(dups) > 0 {
			fmt.Fprintf(os.Stderr, "Error: duplicate --mount base path(s): %v\nEach base path may only be used once.\n", dups)
			os.Exit(2)
		}
		if len(flags.args) > 0 {
			fmt.Fprintln(os.Stderr, "[WARN] Positional OpenAPI spec arguments are ignored when using --mount. Only --mount will be used.")
		}
		mux := http.NewServeMux()
		for _, m := range flags.mounts {
			fmt.Fprintf(os.Stderr, "Loading OpenAPI spec for mount %s: %s...\n", m.BasePath, m.SpecPath)
			d, err := openapi3.NewLoader().LoadFromFile(m.SpecPath)
			if err != nil {
				log.Fatalf("Failed to load OpenAPI spec for %s: %v", m.BasePath, err)
			}
			ops = openapi2mcp.ExtractOpenAPIOperations(d)
			srv := openapi2mcp.NewServerWithOps("openapi-mcp", d.Info.Version, d, ops)
			var handler http.Handler
			if flags.httpTransport == "streamable" {
				handler = openapi2mcp.HandlerForStreamableHTTP(srv, m.BasePath)
			} else {
				handler = openapi2mcp.HandlerForBasePath(srv, m.BasePath)
			}
			mux.Handle(m.BasePath+"/", handler)
			mux.Handle(m.BasePath, handler) // allow both /base and /base/
			fmt.Fprintf(os.Stderr, "Mounted %s at %s\n", m.SpecPath, m.BasePath)
		}
		fmt.Fprintf(os.Stderr, "Starting multi-mount MCP HTTP server on %s...\n", flags.httpAddr)
		if err := http.ListenAndServe(flags.httpAddr, mux); err != nil {
			log.Fatalf("Failed to start MCP HTTP server: %v", err)
		}
		return
	}

	if flags.httpAddr != "" {
		if len(flags.args) != 1 {
			fmt.Fprintln(os.Stderr, "Usage: openapi-mcp --http=:8080 <openapi-spec-path>")
			os.Exit(2)
		}
		specPath := flags.args[0]
		d, err := openapi3.NewLoader().LoadFromFile(specPath)
		if err != nil {
			log.Fatalf("Failed to load OpenAPI spec: %v", err)
		}
		ops := openapi2mcp.ExtractOpenAPIOperations(d)
		srv := openapi2mcp.NewServerWithOps("openapi-mcp", d.Info.Version, d, ops)
		fmt.Fprintf(os.Stderr, "Starting MCP server (HTTP, %s transport) on %s...\n", flags.httpTransport, flags.httpAddr)
		if flags.httpTransport == "streamable" {
			if err := openapi2mcp.ServeStreamableHTTP(srv, flags.httpAddr, "/mcp"); err != nil {
				log.Fatalf("Failed to start MCP HTTP server: %v", err)
			}
		} else {
			if err := openapi2mcp.ServeHTTP(srv, flags.httpAddr, "/mcp"); err != nil {
				log.Fatalf("Failed to start MCP HTTP server: %v", err)
			}
		}
		return
	}

	// stdio mode: require a single positional OpenAPI spec argument
	if len(flags.args) != 1 {
		fmt.Fprintln(os.Stderr, "Usage: openapi-mcp <openapi-spec-path>")
		os.Exit(2)
	}
	specPath := flags.args[0]
	d, err := openapi3.NewLoader().LoadFromFile(specPath)
	if err != nil {
		log.Fatalf("Failed to load OpenAPI spec: %v", err)
	}
	ops = openapi2mcp.ExtractOpenAPIOperations(d)
	srv := openapi2mcp.NewServerWithOps("openapi-mcp", d.Info.Version, d, ops)
	fmt.Fprintln(os.Stderr, "Registered all OpenAPI operations as MCP tools.")
	fmt.Fprintln(os.Stderr, "Starting MCP server (stdio)...")
	if err := openapi2mcp.ServeStdio(srv); err != nil {
		log.Fatalf("Failed to start MCP server: %v", err)
	}
}

// makeMCPHandler returns an http.Handler that serves the MCP server at the given basePath.
func makeMCPHandler(srv *mcpserver.MCPServer, basePath string) http.Handler {
	return openapi2mcp.HandlerForBasePath(srv, basePath)
}

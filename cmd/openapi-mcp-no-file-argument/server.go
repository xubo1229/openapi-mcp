// server_no_openapi_file.go
package main

import (
	"fmt"
	"os"

	"github.com/getkin/kin-openapi/openapi3"
	mcpserver "github.com/jedisct1/openapi-mcp/pkg/mcp/server"
	"github.com/jedisct1/openapi-mcp/pkg/openapi2mcp"
)

// startServer starts the MCP server in stdio or HTTP mode for the no_openapi_file version.
// It registers all OpenAPI operations as MCP tools and starts the server.
func startServer(flags *cliFlags, ops []openapi2mcp.OpenAPIOperation, doc *openapi3.T) {
	if flags.httpAddr != "" {
		// HTTP mode with embedded spec
		srv, logFileHandle := createServerWithOptions("openapi-mcp", doc.Info.Version, doc, ops, flags.logFile, flags.noLogTruncation)
		if logFileHandle != nil {
			defer logFileHandle.Close()
		}
		fmt.Fprintf(os.Stderr, "Starting MCP server (HTTP, %s transport) on %s...\n", flags.httpTransport, flags.httpAddr)
		if flags.httpTransport == "streamable" {
			if err := openapi2mcp.ServeStreamableHTTP(srv, flags.httpAddr, "/mcp"); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to start MCP HTTP server: %v\n", err)
				os.Exit(1)
			}
		} else {
			if err := openapi2mcp.ServeHTTP(srv, flags.httpAddr, "/mcp"); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to start MCP HTTP server: %v\n", err)
				os.Exit(1)
			}
		}
		return
	}

	// stdio mode with embedded spec
	srv, logFileHandle := createServerWithOptions("openapi-mcp", doc.Info.Version, doc, ops, flags.logFile, flags.noLogTruncation)
	if logFileHandle != nil {
		defer logFileHandle.Close()
	}
	fmt.Fprintln(os.Stderr, "Registered all OpenAPI operations as MCP tools.")
	fmt.Fprintln(os.Stderr, "Starting MCP server (stdio)...")
	if err := openapi2mcp.ServeStdio(srv); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start MCP server: %v\n", err)
		os.Exit(1)
	}
}

// createServerWithOptions creates a new MCP server with the given operations and optional logging
func createServerWithOptions(name, version string, doc *openapi3.T, ops []openapi2mcp.OpenAPIOperation, logFile string, noLogTruncation bool) (*mcpserver.MCPServer, *os.File) {
	var opts []mcpserver.ServerOption
	var logFileHandle *os.File

	if logFile != "" {
		hooks, fileHandle, err := createLoggingHooks(logFile, noLogTruncation)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create logging hooks: %v\n", err)
			os.Exit(1)
		}
		logFileHandle = fileHandle
		opts = append(opts, mcpserver.WithHooks(hooks))
		fmt.Fprintf(os.Stderr, "Logging MCP requests and responses to: %s\n", logFile)
	}

	srv := mcpserver.NewMCPServer(name, version, opts...)
	openapi2mcp.RegisterOpenAPITools(srv, ops, doc, nil)
	return srv, logFileHandle
}

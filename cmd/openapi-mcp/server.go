// server.go
package main

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	mcpserver "github.com/jedisct1/openapi-mcp/pkg/mcp/server"
	"github.com/jedisct1/openapi-mcp/pkg/openapi2mcp"
)

// generateAIServerStartupError creates comprehensive, AI-optimized error responses for server startup failures
func generateAIServerStartupError(context string, err error) {
	fmt.Fprintln(os.Stderr, "MCP SERVER STARTUP ERROR")
	fmt.Fprintln(os.Stderr, "=========================")
	fmt.Fprintf(os.Stderr, "\nCONTEXT: %s\n", context)
	fmt.Fprintf(os.Stderr, "ERROR: %v\n\n", err)

	errStr := strings.ToLower(err.Error())

	if strings.Contains(errStr, "bind") || strings.Contains(errStr, "address already in use") {
		fmt.Fprintln(os.Stderr, "ISSUE: Port already in use")
		fmt.Fprintln(os.Stderr, "TROUBLESHOOTING STEPS:")
		fmt.Fprintln(os.Stderr, "1. Check what's using the port: lsof -i :8080")
		fmt.Fprintln(os.Stderr, "2. Kill the process using the port: kill <PID>")
		fmt.Fprintln(os.Stderr, "3. Try a different port: --http=:8081")
		fmt.Fprintln(os.Stderr, "4. Wait a moment and retry (port may be in TIME_WAIT)")
		fmt.Fprintln(os.Stderr, "5. Use a random port: --http=:0")
	} else if strings.Contains(errStr, "permission denied") {
		fmt.Fprintln(os.Stderr, "ISSUE: Permission denied")
		fmt.Fprintln(os.Stderr, "TROUBLESHOOTING STEPS:")
		fmt.Fprintln(os.Stderr, "1. Use a port above 1024 (non-privileged): --http=:8080")
		fmt.Fprintln(os.Stderr, "2. Run with sudo for ports below 1024 (not recommended)")
		fmt.Fprintln(os.Stderr, "3. Check firewall settings")
		fmt.Fprintln(os.Stderr, "4. Verify you have network permissions")
	} else if strings.Contains(errStr, "network") || strings.Contains(errStr, "socket") {
		fmt.Fprintln(os.Stderr, "ISSUE: Network/socket error")
		fmt.Fprintln(os.Stderr, "TROUBLESHOOTING STEPS:")
		fmt.Fprintln(os.Stderr, "1. Check network connectivity")
		fmt.Fprintln(os.Stderr, "2. Verify the address format: --http=:8080 or --http=localhost:8080")
		fmt.Fprintln(os.Stderr, "3. Try binding to localhost specifically: --http=localhost:8080")
		fmt.Fprintln(os.Stderr, "4. Check firewall/security software")
	} else if strings.Contains(errStr, "invalid") || strings.Contains(errStr, "parse") {
		fmt.Fprintln(os.Stderr, "ISSUE: Invalid address format")
		fmt.Fprintln(os.Stderr, "TROUBLESHOOTING STEPS:")
		fmt.Fprintln(os.Stderr, "1. Use correct format: --http=:8080 (port only)")
		fmt.Fprintln(os.Stderr, "2. Or with host: --http=localhost:8080")
		fmt.Fprintln(os.Stderr, "3. Or bind all interfaces: --http=0.0.0.0:8080")
		fmt.Fprintln(os.Stderr, "4. Check for typos in the address")
	} else {
		fmt.Fprintln(os.Stderr, "GENERAL TROUBLESHOOTING:")
		fmt.Fprintln(os.Stderr, "1. Verify the OpenAPI spec file is valid")
		fmt.Fprintln(os.Stderr, "2. Check available system resources (memory, file descriptors)")
		fmt.Fprintln(os.Stderr, "3. Try starting without HTTP: openapi-mcp <spec-file>")
		fmt.Fprintln(os.Stderr, "4. Verify all dependencies are installed")
	}

	fmt.Fprintln(os.Stderr, "\nCOMMON SERVER STARTUP PATTERNS:")
	fmt.Fprintln(os.Stderr, "• stdio mode: openapi-mcp petstore.yaml")
	fmt.Fprintln(os.Stderr, "• HTTP mode: openapi-mcp --http=:8080 petstore.yaml")
	fmt.Fprintln(os.Stderr, "• Specific host: openapi-mcp --http=localhost:8080 petstore.yaml")
	fmt.Fprintln(os.Stderr, "• All interfaces: openapi-mcp --http=0.0.0.0:8080 petstore.yaml")
	fmt.Fprintln(os.Stderr, "• Random port: openapi-mcp --http=:0 petstore.yaml")

	fmt.Fprintln(os.Stderr, "\nDEBUGGING TIPS:")
	fmt.Fprintln(os.Stderr, "• Test with validation first: openapi-mcp validate <spec-file>")
	fmt.Fprintln(os.Stderr, "• Try dry-run mode: openapi-mcp --dry-run <spec-file>")
	fmt.Fprintln(os.Stderr, "• Check network connectivity: curl http://localhost:8080")
	fmt.Fprintln(os.Stderr, "• Monitor system resources: top, ps, netstat")
}

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
				generateAIServerStartupError("Loading OpenAPI spec for mount "+m.BasePath, err)
				os.Exit(1)
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
			generateAIServerStartupError("Starting multi-mount HTTP server on "+flags.httpAddr, err)
			os.Exit(1)
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
			generateAIServerStartupError("Loading OpenAPI spec from "+specPath, err)
			os.Exit(1)
		}
		ops := openapi2mcp.ExtractOpenAPIOperations(d)
		srv := openapi2mcp.NewServerWithOps("openapi-mcp", d.Info.Version, d, ops)
		fmt.Fprintf(os.Stderr, "Starting MCP server (HTTP, %s transport) on %s...\n", flags.httpTransport, flags.httpAddr)
		if flags.httpTransport == "streamable" {
			if err := openapi2mcp.ServeStreamableHTTP(srv, flags.httpAddr, "/mcp"); err != nil {
				generateAIServerStartupError("Starting Streamable HTTP server on "+flags.httpAddr, err)
				os.Exit(1)
			}
		} else {
			if err := openapi2mcp.ServeHTTP(srv, flags.httpAddr, "/mcp"); err != nil {
				generateAIServerStartupError("Starting HTTP server on "+flags.httpAddr, err)
				os.Exit(1)
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
		generateAIServerStartupError("Loading OpenAPI spec from "+specPath, err)
		os.Exit(1)
	}
	ops = openapi2mcp.ExtractOpenAPIOperations(d)
	srv := openapi2mcp.NewServerWithOps("openapi-mcp", d.Info.Version, d, ops)
	fmt.Fprintln(os.Stderr, "Registered all OpenAPI operations as MCP tools.")
	fmt.Fprintln(os.Stderr, "Starting MCP server (stdio)...")
	if err := openapi2mcp.ServeStdio(srv); err != nil {
		generateAIServerStartupError("Starting stdio MCP server", err)
		os.Exit(1)
	}
}

// makeMCPHandler returns an http.Handler that serves the MCP server at the given basePath.
func makeMCPHandler(srv *mcpserver.MCPServer, basePath string) http.Handler {
	return openapi2mcp.HandlerForBasePath(srv, basePath)
}

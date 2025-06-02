// server.go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/jedisct1/openapi-mcp/pkg/mcp/mcp"
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
				fmt.Fprintf(os.Stderr, "Failed to load OpenAPI spec for %s: %v\n", m.BasePath, err)
				os.Exit(1)
			}
			ops = openapi2mcp.ExtractOpenAPIOperations(d)
			srv, logFileHandle := createServerWithOptions("openapi-mcp", d.Info.Version, d, ops, flags.logFile)
			if logFileHandle != nil {
				defer logFileHandle.Close()
			}
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
			fmt.Fprintf(os.Stderr, "Failed to start MCP HTTP server: %v\n", err)
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
			fmt.Fprintf(os.Stderr, "Failed to load OpenAPI spec: %v\n", err)
			os.Exit(1)
		}
		ops := openapi2mcp.ExtractOpenAPIOperations(d)
		srv, logFileHandle := createServerWithOptions("openapi-mcp", d.Info.Version, d, ops, flags.logFile)
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

	// stdio mode: require a single positional OpenAPI spec argument
	if len(flags.args) != 1 {
		fmt.Fprintln(os.Stderr, "Usage: openapi-mcp <openapi-spec-path>")
		os.Exit(2)
	}
	specPath := flags.args[0]
	d, err := openapi3.NewLoader().LoadFromFile(specPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load OpenAPI spec: %v\n", err)
		os.Exit(1)
	}
	ops = openapi2mcp.ExtractOpenAPIOperations(d)
	srv, logFileHandle := createServerWithOptions("openapi-mcp", d.Info.Version, d, ops, flags.logFile)
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

// makeMCPHandler returns an http.Handler that serves the MCP server at the given basePath.
func makeMCPHandler(srv *mcpserver.MCPServer, basePath string) http.Handler {
	return openapi2mcp.HandlerForBasePath(srv, basePath)
}

// createLoggingHooks creates MCP hooks for logging requests and responses to a file
func createLoggingHooks(logFilePath string) (*mcpserver.Hooks, *os.File, error) {
	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open log file: %w", err)
	}

	logger := log.New(logFile, "", 0) // No prefix, we'll format our own timestamps

	hooks := &mcpserver.Hooks{}

	// Log requests
	hooks.AddBeforeAny(func(ctx context.Context, id any, method mcp.MCPMethod, message any) {
		timestamp := time.Now().Format(time.RFC3339)
		reqJSON, _ := json.Marshal(message)
		logEntry := map[string]interface{}{
			"timestamp": timestamp,
			"type":      "request",
			"id":        id,
			"method":    method,
			"message":   json.RawMessage(reqJSON),
		}
		logJSON, _ := json.Marshal(logEntry)
		logger.Println(string(logJSON))
	})

	// Log successful responses
	hooks.AddOnSuccess(func(ctx context.Context, id any, method mcp.MCPMethod, message any, result any) {
		timestamp := time.Now().Format(time.RFC3339)
		resJSON, _ := json.Marshal(result)
		logEntry := map[string]interface{}{
			"timestamp": timestamp,
			"type":      "response",
			"id":        id,
			"method":    method,
			"result":    json.RawMessage(resJSON),
		}
		logJSON, _ := json.Marshal(logEntry)
		logger.Println(string(logJSON))
	})

	// Log errors
	hooks.AddOnError(func(ctx context.Context, id any, method mcp.MCPMethod, message any, err error) {
		timestamp := time.Now().Format(time.RFC3339)
		logEntry := map[string]interface{}{
			"timestamp": timestamp,
			"type":      "error",
			"id":        id,
			"method":    method,
			"error":     err.Error(),
		}
		logJSON, _ := json.Marshal(logEntry)
		logger.Println(string(logJSON))
	})

	return hooks, logFile, nil
}

// createServerWithOptions creates a new MCP server with the given operations and optional logging
func createServerWithOptions(name, version string, doc *openapi3.T, ops []openapi2mcp.OpenAPIOperation, logFile string) (*mcpserver.MCPServer, *os.File) {
	var opts []mcpserver.ServerOption
	var logFileHandle *os.File

	if logFile != "" {
		hooks, fileHandle, err := createLoggingHooks(logFile)
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

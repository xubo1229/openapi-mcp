// server.go
package openapi2mcp

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	mcpserver "github.com/jedisct1/openapi-mcp/pkg/mcp/server"
)

// authContextFunc extracts authentication headers from HTTP requests and sets them
// as environment variables for the duration of each request. This allows API keys
// and other authentication to be provided via HTTP headers when using HTTP mode.
func authContextFunc(ctx context.Context, r *http.Request) context.Context {
	// Save original environment values to restore them later
	origAPIKey := os.Getenv("API_KEY")
	origBearerToken := os.Getenv("BEARER_TOKEN")
	origBasicAuth := os.Getenv("BASIC_AUTH")

	// Extract authentication from HTTP headers
	if apiKey := r.Header.Get("X-API-Key"); apiKey != "" {
		os.Setenv("API_KEY", apiKey)
	} else if apiKey := r.Header.Get("Api-Key"); apiKey != "" {
		os.Setenv("API_KEY", apiKey)
	}

	if bearerToken := r.Header.Get("Authorization"); bearerToken != "" {
		if len(bearerToken) > 7 && bearerToken[:7] == "Bearer " {
			os.Setenv("BEARER_TOKEN", bearerToken[7:])
		} else if len(bearerToken) > 6 && bearerToken[:6] == "Basic " {
			os.Setenv("BASIC_AUTH", bearerToken[6:])
		}
	}

	// Create a context that restores the original environment when done
	return &authContext{
		Context:         ctx,
		origAPIKey:      origAPIKey,
		origBearerToken: origBearerToken,
		origBasicAuth:   origBasicAuth,
	}
}

// authContext wraps a context and restores original environment variables when done
type authContext struct {
	context.Context
	origAPIKey      string
	origBearerToken string
	origBasicAuth   string
}

// Done restores the original environment variables when the context is done
func (c *authContext) Done() <-chan struct{} {
	done := c.Context.Done()
	if done != nil {
		go func() {
			<-done
			c.restoreEnv()
		}()
	}
	return done
}

func (c *authContext) restoreEnv() {
	if c.origAPIKey != "" {
		os.Setenv("API_KEY", c.origAPIKey)
	} else {
		os.Unsetenv("API_KEY")
	}
	if c.origBearerToken != "" {
		os.Setenv("BEARER_TOKEN", c.origBearerToken)
	} else {
		os.Unsetenv("BEARER_TOKEN")
	}
	if c.origBasicAuth != "" {
		os.Setenv("BASIC_AUTH", c.origBasicAuth)
	} else {
		os.Unsetenv("BASIC_AUTH")
	}
}

// NewServer creates a new MCP server, registers all OpenAPI tools, and returns the server.
// Equivalent to calling RegisterOpenAPITools with all operations from the spec.
// Example usage for NewServer:
//
//	doc, _ := openapi2mcp.LoadOpenAPISpec("petstore.yaml")
//	srv := openapi2mcp.NewServer("petstore", doc.Info.Version, doc)
//	openapi2mcp.ServeHTTP(srv, ":8080")
func NewServer(name, version string, doc *openapi3.T) *mcpserver.MCPServer {
	ops := ExtractOpenAPIOperations(doc)
	srv := mcpserver.NewMCPServer(name, version)
	RegisterOpenAPITools(srv, ops, doc, nil)
	return srv
}

// NewServerWithOps creates a new MCP server, registers the provided OpenAPI operations, and returns the server.
// Example usage for NewServerWithOps:
//
//	doc, _ := openapi2mcp.LoadOpenAPISpec("petstore.yaml")
//	ops := openapi2mcp.ExtractOpenAPIOperations(doc)
//	srv := openapi2mcp.NewServerWithOps("petstore", doc.Info.Version, doc, ops)
//	openapi2mcp.ServeHTTP(srv, ":8080")
func NewServerWithOps(name, version string, doc *openapi3.T, ops []OpenAPIOperation) *mcpserver.MCPServer {
	srv := mcpserver.NewMCPServer(name, version)
	RegisterOpenAPITools(srv, ops, doc, nil)
	return srv
}

// ServeStdio starts the MCP server using stdio (wraps mcpserver.ServeStdio).
// Returns an error if the server fails to start.
// Example usage for ServeStdio:
//
//	openapi2mcp.ServeStdio(srv)
func ServeStdio(server *mcpserver.MCPServer) error {
	return mcpserver.ServeStdio(server)
}

// ServeHTTP starts the MCP server using HTTP SSE (wraps mcpserver.NewSSEServer and Start).
// addr is the address to listen on, e.g. ":8080".
// basePath is the base HTTP path to mount the MCP server (e.g. "/mcp").
// Returns an error if the server fails to start.
// Example usage for ServeHTTP:
//
//	srv, _ := openapi2mcp.NewServer("petstore", "1.0.0", doc)
//	openapi2mcp.ServeHTTP(srv, ":8080", "/custom-base")
func ServeHTTP(server *mcpserver.MCPServer, addr string, basePath string) error {
	// Convert the authContextFunc to SSEContextFunc signature
	sseAuthContextFunc := func(ctx context.Context, r *http.Request) context.Context {
		return authContextFunc(ctx, r)
	}

	if basePath == "" {
		basePath = "/mcp"
	}

	sseServer := mcpserver.NewSSEServer(server,
		mcpserver.WithSSEContextFunc(sseAuthContextFunc),
		mcpserver.WithStaticBasePath(basePath),
		mcpserver.WithSSEEndpoint("/sse"),
		mcpserver.WithMessageEndpoint("/message"))
	return sseServer.Start(addr)
}

// Backward-compatible ServeHTTP (defaults to /mcp)
func ServeHTTPDefault(server *mcpserver.MCPServer, addr string) error {
	return ServeHTTP(server, addr, "/mcp")
}

// GetSSEURL returns the URL for establishing an SSE connection to the MCP server.
// baseURL should be the base URL where the server is hosted (e.g., "http://localhost:8080").
// basePath is the base HTTP path (e.g., "/mcp").
// Example usage:
//
//	url := openapi2mcp.GetSSEURL("http://localhost:8080", "/custom-base")
//	// Returns: "http://localhost:8080/custom-base/sse"
func GetSSEURL(baseURL, basePath string) string {
	baseURL = strings.TrimSuffix(baseURL, "/")
	if basePath == "" {
		basePath = "/mcp"
	}
	return baseURL + basePath + "/sse"
}

// Backward-compatible GetSSEURL (defaults to /mcp)
func GetSSEURLDefault(baseURL string) string {
	return GetSSEURL(baseURL, "/mcp")
}

// GetMessageURL returns the URL for sending JSON-RPC requests to the MCP server.
// baseURL should be the base URL where the server is hosted (e.g., "http://localhost:8080").
// basePath is the base HTTP path (e.g., "/mcp").
// sessionID should be the session ID received from the SSE endpoint event.
// Example usage:
//
//	url := openapi2mcp.GetMessageURL("http://localhost:8080", "/custom-base", "session-id-123")
//	// Returns: "http://localhost:8080/custom-base/message?sessionId=session-id-123"
func GetMessageURL(baseURL, basePath, sessionID string) string {
	baseURL = strings.TrimSuffix(baseURL, "/")
	if basePath == "" {
		basePath = "/mcp"
	}
	return fmt.Sprintf("%s%s/message?sessionId=%s", baseURL, basePath, sessionID)
}

// Backward-compatible GetMessageURL (defaults to /mcp)
func GetMessageURLDefault(baseURL, sessionID string) string {
	return GetMessageURL(baseURL, "/mcp", sessionID)
}

// server.go
package openapi2mcp

import (
	"net/http"

	"github.com/getkin/kin-openapi/openapi3"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

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

// ServeHTTP starts the MCP server using HTTP (wraps mcpserver.NewStreamableHTTPServer and http.ListenAndServe).
// addr is the address to listen on, e.g. ":8080".
// Returns an error if the server fails to start.
// Example usage for ServeHTTP:
//
//	srv, _ := openapi2mcp.NewServer("petstore", "1.0.0", doc)
//	openapi2mcp.ServeHTTP(srv, ":8080")
func ServeHTTP(server *mcpserver.MCPServer, addr string) error {
	httpServer := mcpserver.NewStreamableHTTPServer(server)
	return http.ListenAndServe(addr, httpServer)
}

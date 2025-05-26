package client

import (
	"github.com/jedisct1/openapi-mcp/internal/mcp-go/client/transport"
	"github.com/jedisct1/openapi-mcp/internal/mcp-go/server"
)

// NewInProcessClient connect directly to a mcp server object in the same process
func NewInProcessClient(server *server.MCPServer) (*Client, error) {
	inProcessTransport := transport.NewInProcessTransport(server)
	return NewClient(inProcessTransport), nil
}

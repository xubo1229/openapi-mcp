# openapi2mcp Go Library

This package provides a Go library for converting OpenAPI 3.x specifications into MCP (Model Context Protocol) tool servers.

## Installation

```bash
go get github.com/jedisct1/openapi-mcp/pkg/openapi2mcp
```

## Important: Dependency Override Required

Due to internal patches in the mcp-go dependency, you need to add a replace directive to your `go.mod` file:

```go
module your-project

go 1.23

require (
	github.com/jedisct1/openapi-mcp/pkg/openapi2mcp v0.1.0
)

// Override the internal mcp-go dependency with the public version
replace github.com/mark3labs/mcp-go => github.com/mark3labs/mcp-go v0.30.0
```

## Usage

```go
package main

import (
	"log"
	"github.com/jedisct1/openapi-mcp/pkg/openapi2mcp"
)

func main() {
	// Load OpenAPI spec
	doc, err := openapi2mcp.LoadOpenAPISpec("openapi.yaml")
	if err != nil {
		log.Fatal(err)
	}
	
	// Create MCP server
	srv := openapi2mcp.NewServer("myapi", doc.Info.Version, doc)
	
	// Serve over HTTP
	if err := openapi2mcp.ServeHTTP(srv, ":8080"); err != nil {
		log.Fatal(err)
	}
	
	// Or serve over stdio
	// if err := openapi2mcp.ServeStdio(srv); err != nil {
	//     log.Fatal(err)
	// }
}
```

## Features

- Convert OpenAPI 3.x specifications to MCP tool servers
- Support for HTTP and stdio transport
- Automatic tool generation from OpenAPI operations
- Built-in validation and error handling
- AI-optimized responses with structured output

## API Documentation

See [GoDoc](https://pkg.go.dev/github.com/jedisct1/openapi-mcp/pkg/openapi2mcp) for complete API documentation. 
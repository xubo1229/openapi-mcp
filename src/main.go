package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"gopkg.in/yaml.v3"
)

// OpenAPISpec represents the structure of an OpenAPI specification
type OpenAPISpec struct {
	OpenAPI    string                 `yaml:"openapi"`
	Info       map[string]interface{} `yaml:"info"`
	Paths      map[string]interface{} `yaml:"paths"`
	Components map[string]interface{} `yaml:"components"`
}

func main() {
	// Create a new MCP server
	s := server.NewMCPServer(
		"OpenAPI to Go",
		"1.0.0",
		server.WithToolCapabilities(false),
	)

	// Add tool for converting OpenAPI specs to Go
	tool := mcp.NewTool("convert_openapi",
		mcp.WithDescription("Convert OpenAPI spec to an MCP server"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the OpenAPI specification file"),
		),
	)

	// Add tool handler
	s.AddTool(tool, convertHandler)

	// Start the stdio server
	if err := server.ServeStdio(s); err != nil {
		fmt.Printf("Server error: %v\n", err)
	}
}

func convertHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return nil, fmt.Errorf("file_path must be a string")
	}

	// Read the OpenAPI spec file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("error reading file: %v", err)
	}

	// Parse the OpenAPI spec
	var spec OpenAPISpec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("error parsing YAML: %v", err)
	}

	// Basic validation
	if spec.OpenAPI == "" {
		return nil, fmt.Errorf("invalid OpenAPI specification: missing 'openapi' field")
	}

	// Generate a simple report for now
	info, err := json.MarshalIndent(spec.Info, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("error marshaling info: %v", err)
	}

	pathCount := len(spec.Paths)
	
	result := fmt.Sprintf("Successfully parsed OpenAPI specification:\n\nSpec version: %s\nAPI Info: %s\nNumber of paths: %d\n", 
		spec.OpenAPI, info, pathCount)

	return mcp.NewToolResultText(result), nil
}

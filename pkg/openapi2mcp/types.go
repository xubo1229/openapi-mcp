// Package openapi2mcp provides functionality for converting OpenAPI specifications to MCP tools.
// For working with MCP types and tools directly, import github.com/jedisct1/openapi-mcp/pkg/mcp/mcp
// and github.com/jedisct1/openapi-mcp/pkg/mcp/server
package openapi2mcp

// LintIssue represents a single linting issue found in an OpenAPI spec
type LintIssue struct {
	Type       string `json:"type"`                // "error" or "warning"
	Message    string `json:"message"`             // The main error/warning message
	Suggestion string `json:"suggestion"`          // Actionable suggestion for fixing the issue
	Operation  string `json:"operation,omitempty"` // Operation ID where the issue was found
	Path       string `json:"path,omitempty"`      // API path where the issue was found
	Method     string `json:"method,omitempty"`    // HTTP method where the issue was found
	Parameter  string `json:"parameter,omitempty"` // Parameter name where the issue was found
	Field      string `json:"field,omitempty"`     // Specific field where the issue was found
}

// LintResult represents the result of linting or validating an OpenAPI spec
type LintResult struct {
	Success      bool        `json:"success"`           // Whether the linting/validation passed
	ErrorCount   int         `json:"error_count"`       // Number of errors found
	WarningCount int         `json:"warning_count"`     // Number of warnings found
	Issues       []LintIssue `json:"issues"`            // List of all issues found
	Summary      string      `json:"summary,omitempty"` // Summary message
}

// HTTPLintRequest represents the request body for HTTP lint/validate endpoints
type HTTPLintRequest struct {
	OpenAPISpec string `json:"openapi_spec"` // The OpenAPI spec as a YAML or JSON string
}

// Package openapi2mcp provides type aliases and helpers for working with MCP tools and properties.
// These aliases and helpers make it easier to construct and configure tools using the MCP protocol.
package openapi2mcp

import (
	"encoding/json"

	"github.com/jedisct1/openapi-mcp/pkg/mcp/mcp"
	mcpserver "github.com/jedisct1/openapi-mcp/pkg/mcp/server"
)

// Tool is an alias for mcp.Tool, representing the definition of an MCP tool.
type Tool = mcp.Tool

// ToolOption is an alias for mcp.ToolOption, a function that configures a Tool.
type ToolOption = mcp.ToolOption

// PropertyOption is an alias for mcp.PropertyOption, a function that configures a property in a Tool's input schema.
type PropertyOption = mcp.PropertyOption

// ToolHandlerFunc is an alias for mcpserver.ToolHandlerFunc, a function that handles tool calls.
type ToolHandlerFunc = mcpserver.ToolHandlerFunc

// Tools provides functions for creating and configuring MCP tools.
// These are convenient wrappers around the core MCP tool construction functions.
var Tools = struct {
	New                           func(name string, opts ...mcp.ToolOption) mcp.Tool
	NewWithRawSchema              func(name, description string, schema json.RawMessage) mcp.Tool
	WithDescription               func(description string) mcp.ToolOption
	WithToolAnnotation            func(annotation mcp.ToolAnnotation) mcp.ToolOption
	WithTitleAnnotation           func(title string) mcp.ToolOption
	WithReadOnlyHintAnnotation    func(value bool) mcp.ToolOption
	WithDestructiveHintAnnotation func(value bool) mcp.ToolOption
	WithIdempotentHintAnnotation  func(value bool) mcp.ToolOption
	WithOpenWorldHintAnnotation   func(value bool) mcp.ToolOption
}{
	New:                           mcp.NewTool,
	NewWithRawSchema:              mcp.NewToolWithRawSchema,
	WithDescription:               mcp.WithDescription,
	WithToolAnnotation:            mcp.WithToolAnnotation,
	WithTitleAnnotation:           mcp.WithTitleAnnotation,
	WithReadOnlyHintAnnotation:    mcp.WithReadOnlyHintAnnotation,
	WithDestructiveHintAnnotation: mcp.WithDestructiveHintAnnotation,
	WithIdempotentHintAnnotation:  mcp.WithIdempotentHintAnnotation,
	WithOpenWorldHintAnnotation:   mcp.WithOpenWorldHintAnnotation,
}

// Properties provides functions for configuring tool properties.
// These are convenient wrappers around the core MCP property construction functions.
var Properties = struct {
	WithString           func(name string, opts ...mcp.PropertyOption) mcp.ToolOption
	WithNumber           func(name string, opts ...mcp.PropertyOption) mcp.ToolOption
	WithBoolean          func(name string, opts ...mcp.PropertyOption) mcp.ToolOption
	WithObject           func(name string, opts ...mcp.PropertyOption) mcp.ToolOption
	WithArray            func(name string, opts ...mcp.PropertyOption) mcp.ToolOption
	Description          func(description string) mcp.PropertyOption
	Required             func() mcp.PropertyOption
	Enum                 func(values ...string) mcp.PropertyOption
	DefaultString        func(value string) mcp.PropertyOption
	DefaultNumber        func(value float64) mcp.PropertyOption
	DefaultBool          func(value bool) mcp.PropertyOption
	DefaultArray         func(value []any) mcp.PropertyOption
	MaxLength            func(max int) mcp.PropertyOption
	MinLength            func(min int) mcp.PropertyOption
	Pattern              func(pattern string) mcp.PropertyOption
	Max                  func(max float64) mcp.PropertyOption
	Min                  func(min float64) mcp.PropertyOption
	MultipleOf           func(value float64) mcp.PropertyOption
	Properties           func(props map[string]any) mcp.PropertyOption
	AdditionalProperties func(schema any) mcp.PropertyOption
	MinProperties        func(min int) mcp.PropertyOption
	MaxProperties        func(max int) mcp.PropertyOption
	PropertyNames        func(schema map[string]any) mcp.PropertyOption
	Items                func(schema any) mcp.PropertyOption
	MinItems             func(min int) mcp.PropertyOption
	MaxItems             func(max int) mcp.PropertyOption
	UniqueItems          func(unique bool) mcp.PropertyOption
}{
	WithString:           mcp.WithString,
	WithNumber:           mcp.WithNumber,
	WithBoolean:          mcp.WithBoolean,
	WithObject:           mcp.WithObject,
	WithArray:            mcp.WithArray,
	Description:          mcp.Description,
	Required:             mcp.Required,
	Enum:                 mcp.Enum,
	DefaultString:        mcp.DefaultString,
	DefaultNumber:        mcp.DefaultNumber,
	DefaultBool:          mcp.DefaultBool,
	DefaultArray:         mcp.DefaultArray[any],
	MaxLength:            mcp.MaxLength,
	MinLength:            mcp.MinLength,
	Pattern:              mcp.Pattern,
	Max:                  mcp.Max,
	Min:                  mcp.Min,
	MultipleOf:           mcp.MultipleOf,
	Properties:           mcp.Properties,
	AdditionalProperties: mcp.AdditionalProperties,
	MinProperties:        mcp.MinProperties,
	MaxProperties:        mcp.MaxProperties,
	PropertyNames:        mcp.PropertyNames,
	Items:                mcp.Items,
	MinItems:             mcp.MinItems,
	MaxItems:             mcp.MaxItems,
	UniqueItems:          mcp.UniqueItems,
}

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

package main

import (
	"fmt"
	"os"
	"regexp"

	"github.com/jedisct1/openapi-mcp/pkg/openapi2mcp"
)

// main is the entrypoint for the openapi-mcp CLI.
// It parses flags, loads the OpenAPI spec, and dispatches to the appropriate mode (server, doc, dry-run, etc).
func main() {
	flags := parseFlags()

	if flags.showHelp {
		printHelp()
		os.Exit(0)
	}

	// Set env vars from flags if provided
	setEnvFromFlags(flags)

	args := flags.args
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Error: missing required <openapi-spec-path> argument.")
		printHelp()
		os.Exit(1)
	}

	// Enforce: --lint (and all flags) must come before 'validate' command
	for i, arg := range os.Args[1:] {
		if arg == "validate" {
			for _, after := range os.Args[i+2:] {
				if after == "--lint" {
					fmt.Fprintln(os.Stderr, "Error: --lint must be specified before the 'validate' command.")
					fmt.Fprintln(os.Stderr, "Usage: openapi-mcp --lint validate <openapi-spec-path>")
					os.Exit(1)
				}
			}
		}
	}

	// --- Validate subcommand ---
	if args[0] == "validate" {
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Error: missing required <openapi-spec-path> argument for validate.")
			os.Exit(1)
		}
		specPath := args[1]
		doc, err := openapi2mcp.LoadOpenAPISpec(specPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Validation failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintln(os.Stderr, "OpenAPI spec loaded and validated successfully.")
		// Run MCP self-test for actionable errors
		// We'll simulate tool names as if all operationIds are present
		ops := openapi2mcp.ExtractOpenAPIOperations(doc)
		var toolNames []string
		for _, op := range ops {
			toolNames = append(toolNames, op.OperationID)
		}
		err = openapi2mcp.SelfTestOpenAPIMCPWithOptions(doc, toolNames, false)
		if err != nil {
			fmt.Fprintf(os.Stderr, "MCP self-test failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintln(os.Stderr, "MCP self-test passed: all tools and required arguments are present.")
		os.Exit(0)
	}
	// --- End validate subcommand ---

	// --- Lint subcommand ---
	if args[0] == "lint" {
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Error: missing required <openapi-spec-path> argument for lint.")
			os.Exit(1)
		}
		specPath := args[1]
		doc, err := openapi2mcp.LoadOpenAPISpec(specPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Linting failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintln(os.Stderr, "OpenAPI spec loaded successfully.")
		// Run detailed MCP linting with comprehensive suggestions
		ops := openapi2mcp.ExtractOpenAPIOperations(doc)
		var toolNames []string
		for _, op := range ops {
			toolNames = append(toolNames, op.OperationID)
		}
		err = openapi2mcp.SelfTestOpenAPIMCPWithOptions(doc, toolNames, true)
		if err != nil {
			fmt.Fprintf(os.Stderr, "OpenAPI linting completed with issues: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintln(os.Stderr, "OpenAPI linting passed: spec follows all best practices.")
		os.Exit(0)
	}
	// --- End lint subcommand ---

	specPath := args[len(args)-1]
	doc, err := openapi2mcp.LoadOpenAPISpec(specPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Could not load OpenAPI spec: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr, "OpenAPI spec loaded and validated successfully.")

	// Compile regex filters if provided
	var includeRegex, excludeRegex *regexp.Regexp
	if val := os.Getenv("INCLUDE_DESC_REGEX"); val != "" {
		includeRegex, err = regexp.Compile(val)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Invalid INCLUDE_DESC_REGEX: %v\n", err)
			os.Exit(1)
		}
	}
	if val := os.Getenv("EXCLUDE_DESC_REGEX"); val != "" {
		excludeRegex, err = regexp.Compile(val)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Invalid EXCLUDE_DESC_REGEX: %v\n", err)
			os.Exit(1)
		}
	}

	ops := openapi2mcp.ExtractFilteredOpenAPIOperations(doc, includeRegex, excludeRegex)

	// Dispatch to doc, dry-run, or server mode
	if flags.docFile != "" {
		handleDocMode(flags, ops, doc)
		return
	}
	if flags.dryRun {
		handleDryRunMode(flags, ops, doc)
		return
	}
	startServer(flags, ops, doc)
}

// handleDocMode handles the --doc mode, generating documentation for all tools.
// func handleDocMode(flags *cliFlags, ops []openapi2mcp.OpenAPIOperation, doc *openapi3.T) {
// 	// Implementation in doc.go
// 	panic("handleDocMode not yet implemented")
// }

// handleDryRunMode handles the --dry-run mode, printing tool schemas and summaries.
// func handleDryRunMode(flags *cliFlags, ops []openapi2mcp.OpenAPIOperation, doc *openapi3.T) {
// 	// Implementation in utils.go or a dedicated file
// 	panic("handleDryRunMode not yet implemented")
// }

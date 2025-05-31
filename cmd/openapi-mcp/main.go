package main

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/jedisct1/openapi-mcp/pkg/openapi2mcp"
	"gopkg.in/yaml.v3"
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

	// If --mount is used with --http, do not require a positional argument
	if flags.httpAddr != "" && len(flags.mounts) > 0 {
		if len(args) > 0 {
			fmt.Fprintln(os.Stderr, "[WARN] Positional OpenAPI spec arguments are ignored when using --mount. Only --mount will be used.")
		}
		startServer(flags, nil, nil)
		return
	}

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
		// Check if HTTP mode is requested
		if flags.httpAddr != "" {
			fmt.Fprintf(os.Stderr, "Starting OpenAPI validation HTTP server on %s\n", flags.httpAddr)
			err := openapi2mcp.ServeHTTPLint(flags.httpAddr, false)
			if err != nil {
				fmt.Fprintf(os.Stderr, "HTTP server failed: %v\n", err)
				os.Exit(1)
			}
			return
		}

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
		// Check if HTTP mode is requested
		if flags.httpAddr != "" {
			fmt.Fprintf(os.Stderr, "Starting OpenAPI linting HTTP server on %s\n", flags.httpAddr)
			err := openapi2mcp.ServeHTTPLint(flags.httpAddr, true)
			if err != nil {
				fmt.Fprintf(os.Stderr, "HTTP server failed: %v\n", err)
				os.Exit(1)
			}
			return
		}

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

	// --- Filter subcommand ---
	if args[0] == "filter" {
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Error: missing required <openapi-spec-path> argument for filter.")
			os.Exit(1)
		}
		specPath := args[1]
		doc, err := openapi2mcp.LoadOpenAPISpec(specPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Could not load OpenAPI spec: %v\n", err)
			os.Exit(1)
		}

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
		// Apply tag filter if present
		if len(flags.tagFlags) > 0 {
			var filtered []openapi2mcp.OpenAPIOperation
			for _, op := range ops {
				found := false
				for _, tag := range op.Tags {
					for _, want := range flags.tagFlags {
						if tag == want {
							found = true
							break
						}
					}
					if found {
						break
					}
				}
				if found {
					filtered = append(filtered, op)
				}
			}
			ops = filtered
		}
		// Apply function list file filter if present
		if flags.functionListFile != "" {
			funcNames := make(map[string]struct{})
			data, err := os.ReadFile(flags.functionListFile)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: Could not read function list file: %v\n", err)
				os.Exit(1)
			}
			for _, line := range regexp.MustCompile(`\r?\n`).Split(string(data), -1) {
				line = regexp.MustCompile(`^\s+|\s+$`).ReplaceAllString(line, "")
				if line != "" {
					funcNames[line] = struct{}{}
				}
			}
			var filtered []openapi2mcp.OpenAPIOperation
			for _, op := range ops {
				if _, ok := funcNames[op.OperationID]; ok {
					filtered = append(filtered, op)
				}
			}
			ops = filtered
		}
		// Output the filtered OpenAPI spec as a valid OpenAPI file using kin-openapi's marshaling
		ext := ""
		if dot := len(specPath) - 1 - len(specPath); dot >= 0 {
			ext = ""
		} else {
			dot = len(specPath) - 1
			for i := len(specPath) - 1; i >= 0; i-- {
				if specPath[i] == '.' {
					dot = i
					break
				}
			}
			if dot < len(specPath)-1 {
				ext = specPath[dot+1:]
			}
		}
		ext = strings.ToLower(ext)
		if ext == "yaml" || ext == "yml" {
			// Output as YAML using kin-openapi's MarshalYAML
			yamlVal, err := doc.MarshalYAML()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: Failed to marshal OpenAPI as YAML: %v\n", err)
				os.Exit(1)
			}
			switch v := yamlVal.(type) {
			case []byte:
				fmt.Print(string(v))
			default:
				// Fallback: use yaml.v3 Marshal if needed
				b, err := yaml.Marshal(v)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: Failed to marshal YAML fallback: %v\n", err)
					os.Exit(1)
				}
				fmt.Print(string(b))
			}
		} else {
			// Output as JSON using encoding/json
			jsonBytes, err := json.MarshalIndent(doc, "", "  ")
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: Failed to marshal OpenAPI as JSON: %v\n", err)
				os.Exit(1)
			}
			fmt.Println(string(jsonBytes))
		}
		os.Exit(0)
	}

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

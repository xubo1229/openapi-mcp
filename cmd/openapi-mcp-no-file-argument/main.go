package main

import (
	"embed"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/jedisct1/openapi-mcp/pkg/openapi2mcp"
	"gopkg.in/yaml.v3"
)

//go:embed specs/*
var specFiles embed.FS

// loadOpenAPISpecsFromEmbedded loads all OpenAPI specs from the embedded filesystem
func loadOpenAPISpecsFromEmbedded() ([]*openapi3.T, error) {
	var docs []*openapi3.T
	var errors []error

	// Read all files from the embedded specs directory
	entries, err := specFiles.ReadDir("specs")
	if err != nil {
		return nil, fmt.Errorf("failed to read embedded specs directory: %v", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Read the file content
		data, err := specFiles.ReadFile("specs/" + entry.Name())
		if err != nil {
			errors = append(errors, fmt.Errorf("failed to read embedded file %s: %v", entry.Name(), err))
			continue
		}

		// Load spec from bytes
		doc, err := openapi2mcp.LoadOpenAPISpecFromBytes(data)
		if err != nil {
			errors = append(errors, fmt.Errorf("failed to load spec from embedded file %s: %v", entry.Name(), err))
			continue
		}

		docs = append(docs, doc)
	}

	if len(errors) > 0 {
		// If we have both docs and errors, return the docs but with a warning
		if len(docs) > 0 {
			return docs, fmt.Errorf("some embedded specs failed to load: %v", errors)
		}
		return docs, fmt.Errorf("all embedded specs failed to load: %v", errors)
	}

	// If no specs were loaded, return an error
	if len(docs) == 0 {
		return nil, fmt.Errorf("no OpenAPI specs found in embedded filesystem")
	}

	return docs, nil
}

// collectUsedSchemas traverses the OpenAPI document and collects all schema names that are referenced
func collectUsedSchemas(doc *openapi3.T) map[string]bool {
	used := make(map[string]bool)

	// Helper function to extract schema name from $ref
	extractSchemaName := func(ref string) string {
		if strings.HasPrefix(ref, "#/components/schemas/") {
			return strings.TrimPrefix(ref, "#/components/schemas/")
		}
		return ""
	}

	// Helper function to recursively collect refs from a schema
	var collectRefsFromSchema func(*openapi3.SchemaRef)
	collectRefsFromSchema = func(schemaRef *openapi3.SchemaRef) {
		if schemaRef == nil {
			return
		}

		// Check if this is a reference
		if schemaRef.Ref != "" {
			if name := extractSchemaName(schemaRef.Ref); name != "" {
				if !used[name] {
					used[name] = true
					// Recursively check the referenced schema
					if doc.Components != nil && doc.Components.Schemas != nil {
						if refSchema, exists := doc.Components.Schemas[name]; exists {
							collectRefsFromSchema(refSchema)
						}
					}
				}
			}
			return
		}

		// Check the schema value itself
		if schemaRef.Value != nil {
			schema := schemaRef.Value

			// Check properties
			for _, propRef := range schema.Properties {
				collectRefsFromSchema(propRef)
			}

			// Check items (for arrays)
			if schema.Items != nil {
				collectRefsFromSchema(schema.Items)
			}

			// Check additionalProperties
			if schema.AdditionalProperties.Schema != nil {
				collectRefsFromSchema(schema.AdditionalProperties.Schema)
			}

			// Check allOf, anyOf, oneOf
			for _, ref := range schema.AllOf {
				collectRefsFromSchema(ref)
			}
			for _, ref := range schema.AnyOf {
				collectRefsFromSchema(ref)
			}
			for _, ref := range schema.OneOf {
				collectRefsFromSchema(ref)
			}

			// Check not
			if schema.Not != nil {
				collectRefsFromSchema(schema.Not)
			}
		}
	}

	// Traverse all paths and operations
	if doc.Paths != nil {
		for _, pathItem := range doc.Paths.Map() {
			// Check parameters at path level
			for _, paramRef := range pathItem.Parameters {
				if paramRef != nil && paramRef.Value != nil && paramRef.Value.Schema != nil {
					collectRefsFromSchema(paramRef.Value.Schema)
				}
			}

			// Check each operation
			for _, op := range pathItem.Operations() {
				if op == nil {
					continue
				}

				// Check parameters
				for _, paramRef := range op.Parameters {
					if paramRef != nil && paramRef.Value != nil && paramRef.Value.Schema != nil {
						collectRefsFromSchema(paramRef.Value.Schema)
					}
				}

				// Check request body
				if op.RequestBody != nil && op.RequestBody.Value != nil {
					for _, mediaType := range op.RequestBody.Value.Content {
						if mediaType.Schema != nil {
							collectRefsFromSchema(mediaType.Schema)
						}
					}
				}

				// Check responses
				if op.Responses != nil {
					for _, respRef := range op.Responses.Map() {
						if respRef != nil && respRef.Value != nil {
							for _, mediaType := range respRef.Value.Content {
								if mediaType.Schema != nil {
									collectRefsFromSchema(mediaType.Schema)
								}
							}
						}
					}
				}
			}
		}
	}

	return used
}

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

	// For HTTP mode, load specs and start server
	if flags.httpAddr != "" {
		// Load specs for HTTP mode
		docs, err := loadOpenAPISpecsFromEmbedded()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Could not load embedded OpenAPI spec: %v\n", err)
			os.Exit(1)
		}

		// Merge specs if multiple found
		var doc *openapi3.T
		if len(docs) > 1 {
			fmt.Fprintf(os.Stderr, "Multiple OpenAPI specs found (%d), merging...\n", len(docs))
			doc, err = openapi2mcp.MergeOpenAPISpecs(docs)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to merge specs: %v\n", err)
				// Fall back to using the first spec
				doc = docs[0]
				fmt.Fprintf(os.Stderr, "Using first spec only.\n")
			}
		} else {
			doc = docs[0]
		}

		ops := openapi2mcp.ExtractFilteredOpenAPIOperations(doc, nil, nil)
		startServer(flags, ops, doc)
		return
	}

	// Ignore any additional arguments - only flags are processed
	if len(args) >= 1 && (args[0] == "validate" || args[0] == "lint" || args[0] == "filter") {
		// Process subcommands with no additional arguments
		if len(args) >= 2 {
			ignoredArgs := make([]string, len(args)-1)
			for i := 1; i < len(args); i++ {
				ignoredArgs[i-1] = args[i]
			}
			fmt.Fprintf(os.Stderr, "[INFO] Ignoring additional arguments: %v\n", ignoredArgs)
		}
	} else if len(args) >= 1 {
		// Ignore any positional arguments in server mode
		ignoredArgs := make([]string, len(args))
		for i := 0; i < len(args); i++ {
			ignoredArgs[i] = args[i]
		}
		fmt.Fprintf(os.Stderr, "[INFO] Ignoring positional arguments: %v\n", ignoredArgs)
	}

	// Enforce: --lint (and all flags) must come before 'validate' command
	for i, arg := range os.Args[1:] {
		if arg == "validate" {
			for _, after := range os.Args[i+2:] {
				if after == "--lint" {
					fmt.Fprintln(os.Stderr, "Error: --lint must be specified before the 'validate' command.")
					fmt.Fprintln(os.Stderr, "Usage: openapi-mcp --lint validate")
					os.Exit(1)
				}
			}
		}
	}

	// --- Validate subcommand ---
	if len(args) > 0 && args[0] == "validate" {
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

		// For embedded version, always use the embedded spec
		// Load multiple specs if present
		docs, err := loadOpenAPISpecsFromEmbedded()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Validation failed: %v\n", err)
			// Raw OpenAPI spec content logging removed for embedded FS approach
			os.Exit(1)
		}

		// Merge specs if multiple found
		var doc *openapi3.T
		if len(docs) > 1 {
			fmt.Fprintf(os.Stderr, "Multiple OpenAPI specs found (%d), merging...\n", len(docs))
			doc, err = openapi2mcp.MergeOpenAPISpecs(docs)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to merge specs: %v\n", err)
				// Fall back to using the first spec
				doc = docs[0]
				fmt.Fprintf(os.Stderr, "Using first spec only.\n")
			}
		} else {
			doc = docs[0]
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
	if len(args) > 0 && args[0] == "lint" {
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

		// For embedded version, always use the embedded spec
		// Load multiple specs if present
		docs, err := loadOpenAPISpecsFromEmbedded()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Linting failed: %v\n", err)
			// Raw OpenAPI spec content logging removed for embedded FS approach
			os.Exit(1)
		}

		// Merge specs if multiple found
		var doc *openapi3.T
		if len(docs) > 1 {
			fmt.Fprintf(os.Stderr, "Multiple OpenAPI specs found (%d), merging...\n", len(docs))
			doc, err = openapi2mcp.MergeOpenAPISpecs(docs)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to merge specs: %v\n", err)
				// Fall back to using the first spec
				doc = docs[0]
				fmt.Fprintf(os.Stderr, "Using first spec only.\n")
			}
		} else {
			doc = docs[0]
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
	if len(args) > 0 && args[0] == "filter" {
		// For embedded version, always use the embedded spec
		// Load multiple specs if present
		docs, err := loadOpenAPISpecsFromEmbedded()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Filter operation failed: %v\n", err)
			// Raw OpenAPI spec content logging removed for embedded FS approach
			os.Exit(1)
		}

		// Merge specs if multiple found
		var doc *openapi3.T
		if len(docs) > 1 {
			fmt.Fprintf(os.Stderr, "Multiple OpenAPI specs found (%d), merging...\n", len(docs))
			doc, err = openapi2mcp.MergeOpenAPISpecs(docs)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to merge specs: %v\n", err)
				// Fall back to using the first spec
				doc = docs[0]
				fmt.Fprintf(os.Stderr, "Using first spec only.\n")
			}
		} else {
			doc = docs[0]
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

		// Patch doc.Paths to only include filtered operations
		if len(ops) == 0 {
			// If no operations remain after filtering, clear all paths
			for path := range doc.Paths.Map() {
				doc.Paths.Delete(path)
			}
		} else {
			opMap := make(map[string]map[string]struct{}) // path -> method -> present
			for _, op := range ops {
				if _, ok := opMap[op.Path]; !ok {
					opMap[op.Path] = make(map[string]struct{})
				}
				opMap[op.Path][strings.ToLower(op.Method)] = struct{}{}
			}
			for path, pathItem := range doc.Paths.Map() {
				// Remove methods not in opMap
				for method := range pathItem.Operations() {
					if _, ok := opMap[path][strings.ToLower(method)]; !ok {
						// Remove this method from the PathItem
						switch strings.ToLower(method) {
						case "get":
							pathItem.Get = nil
						case "put":
							pathItem.Put = nil
						case "post":
							pathItem.Post = nil
						case "delete":
							pathItem.Delete = nil
						case "options":
							pathItem.Options = nil
						case "head":
							pathItem.Head = nil
						case "patch":
							pathItem.Patch = nil
						case "trace":
							pathItem.Trace = nil
						}
					}
				}
				// If all methods are nil, remove the path entirely
				hasOp := false
				for _, op := range pathItem.Operations() {
					if op != nil {
						hasOp = true
						break
					}
				}
				if !hasOp {
					doc.Paths.Delete(path)
				}
			}
		}

		// Clean up unused components/schemas
		if doc.Components != nil && doc.Components.Schemas != nil {
			usedSchemas := collectUsedSchemas(doc)
			// Remove unused schemas
			for schemaName := range doc.Components.Schemas {
				if _, used := usedSchemas[schemaName]; !used {
					delete(doc.Components.Schemas, schemaName)
				}
			}
		}

		// Output the filtered OpenAPI spec as a valid OpenAPI file using kin-openapi's marshaling
		// For embedded version, we'll default to YAML format
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
		os.Exit(0)
	}

	// For embedded version, load the OpenAPI spec from embedded data
	// Load multiple specs if present
	docs, err := loadOpenAPISpecsFromEmbedded()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Could not load embedded OpenAPI spec: %v\n", err)
		// Raw OpenAPI spec content logging removed for embedded FS approach
		os.Exit(1)
	}

	// Merge specs if multiple found
	var doc *openapi3.T
	if len(docs) > 1 {
		fmt.Fprintf(os.Stderr, "Multiple OpenAPI specs found (%d), merging...\n", len(docs))
		doc, err = openapi2mcp.MergeOpenAPISpecs(docs)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to merge specs: %v\n", err)
			// Fall back to using the first spec
			doc = docs[0]
			fmt.Fprintf(os.Stderr, "Using first spec only.\n")
		}
	} else {
		doc = docs[0]
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

// flags.go
package main

import (
	"flag"
	"fmt"
	"os"
)

// cliFlags holds all parsed CLI flags and arguments.
type cliFlags struct {
	showHelp           bool
	extended           bool
	quiet              bool
	machine            bool
	apiKeyFlag         string
	baseURLFlag        string
	bearerToken        string
	basicAuth          string
	httpAddr           string
	includeDescRegex   string
	excludeDescRegex   string
	dryRun             bool
	summary            bool
	toolNameFormat     string
	diffFile           string
	tagFlags           multiFlag
	docFile            string
	docFormat          string
	postHookCmd        string
	noConfirmDangerous bool
	args               []string
}

// parseFlags parses all CLI flags and returns a cliFlags struct.
func parseFlags() *cliFlags {
	var flags cliFlags
	flag.BoolVar(&flags.showHelp, "h", false, "Show help")
	flag.BoolVar(&flags.showHelp, "help", false, "Show help")
	flag.BoolVar(&flags.extended, "extended", false, "Enable extended (human-friendly) output")
	// Default to minimal output
	flags.quiet = true
	flags.machine = true
	flag.StringVar(&flags.apiKeyFlag, "api-key", "", "API key for authenticated endpoints (overrides API_KEY env)")
	flag.StringVar(&flags.baseURLFlag, "base-url", "", "Override the base URL for HTTP calls (overrides OPENAPI_BASE_URL env)")
	flag.StringVar(&flags.bearerToken, "bearer-token", os.Getenv("BEARER_TOKEN"), "Bearer token for Authorization header (overrides BEARER_TOKEN env)")
	flag.StringVar(&flags.basicAuth, "basic-auth", os.Getenv("BASIC_AUTH"), "Basic auth (user:pass) for Authorization header (overrides BASIC_AUTH env)")
	flag.StringVar(&flags.httpAddr, "http", "", "If set, serve MCP over HTTP on this address (e.g., :8080) instead of stdio.")
	flag.StringVar(&flags.includeDescRegex, "include-desc-regex", "", "Only include APIs whose description matches this regex (overrides INCLUDE_DESC_REGEX env)")
	flag.StringVar(&flags.excludeDescRegex, "exclude-desc-regex", "", "Exclude APIs whose description matches this regex (overrides EXCLUDE_DESC_REGEX env)")
	flag.BoolVar(&flags.dryRun, "dry-run", false, "Print the generated MCP tool schemas and exit (do not start the server)")
	flag.Var(&flags.tagFlags, "tag", "Only include tools with the given OpenAPI tag (repeatable)")
	flag.StringVar(&flags.toolNameFormat, "tool-name-format", "", "Format tool names: lower, upper, snake, camel")
	flag.BoolVar(&flags.summary, "summary", false, "Print a summary of the generated tools (count, tags, etc)")
	flag.StringVar(&flags.diffFile, "diff", "", "Compare the generated output to a previous run (file path)")
	flag.StringVar(&flags.docFile, "doc", "", "Write Markdown/HTML documentation for all tools to this file (implies no server)")
	flag.StringVar(&flags.docFormat, "doc-format", "markdown", "Documentation format: markdown (default) or html")
	flag.StringVar(&flags.postHookCmd, "post-hook-cmd", "", "Command to post-process the generated tool schema JSON (used in --dry-run or --doc mode)")
	flag.BoolVar(&flags.noConfirmDangerous, "no-confirm-dangerous", false, "Disable confirmation prompt for dangerous (PUT/POST/DELETE) actions in tool descriptions")
	flag.Parse()
	flags.args = flag.Args()
	if flags.extended {
		flags.quiet = false
		flags.machine = false
	}
	return &flags
}

// setEnvFromFlags sets environment variables from CLI flags if provided.
func setEnvFromFlags(flags *cliFlags) {
	if flags.apiKeyFlag != "" {
		os.Setenv("API_KEY", flags.apiKeyFlag)
	}
	if flags.baseURLFlag != "" {
		os.Setenv("OPENAPI_BASE_URL", flags.baseURLFlag)
	}
	if flags.includeDescRegex != "" {
		os.Setenv("INCLUDE_DESC_REGEX", flags.includeDescRegex)
	}
	if flags.excludeDescRegex != "" {
		os.Setenv("EXCLUDE_DESC_REGEX", flags.excludeDescRegex)
	}
}

// printHelp prints the CLI help message.
func printHelp() {
	fmt.Print(`openapi-mcp: Expose OpenAPI APIs as MCP tools

Usage:
  openapi-mcp [flags] <openapi-spec-path>
  openapi-mcp validate <openapi-spec-path>

Commands:
  validate <openapi-spec-path>  Validate the OpenAPI spec and report actionable errors (does not start a server)

Flags:
  --extended           Enable extended (human-friendly) output (default: minimal/agent)
  --api-key            API key for authenticated endpoints
  --base-url           Override the base URL for HTTP calls
  --bearer-token       Bearer token for Authorization header
  --basic-auth         Basic auth (user:pass) for Authorization header
  --http               Serve MCP over HTTP instead of stdio
  --include-desc-regex Only include APIs whose description matches this regex
  --exclude-desc-regex Exclude APIs whose description matches this regex
  --dry-run            Print the generated MCP tool schemas as JSON and exit
  --doc                Write Markdown/HTML documentation for all tools to this file
  --doc-format         Documentation format: markdown (default) or html
  --post-hook-cmd      Command to post-process the generated tool schema JSON
  --no-confirm-dangerous Disable confirmation for dangerous actions
  --summary            Print a summary for CI
  --tag                Only include tools with the given tag
  --diff               Compare generated tools with a reference file
  --help, -h           Show help

By default, output is minimal and agent-friendly. Use --extended for banners, help, and human-readable output.
`)
	os.Exit(0)
}

// multiFlag is a custom flag type for collecting repeated string values.
type multiFlag []string

// String returns the string representation of the multiFlag.
func (m *multiFlag) String() string {
	return fmt.Sprintf("%v", *m)
}

// Set appends a value to the multiFlag.
func (m *multiFlag) Set(val string) error {
	*m = append(*m, val)
	return nil
}

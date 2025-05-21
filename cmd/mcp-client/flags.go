// flags.go
package main

import (
	"flag"
	"fmt"
	"os"
)

// cliFlags holds all parsed CLI flags and arguments for mcp-client.
type cliFlags struct {
	showHelp bool
	quiet    bool
	machine  bool
	args     []string
}

// parseFlags parses all CLI flags and returns a cliFlags struct.
func parseFlags() *cliFlags {
	var flags cliFlags
	flag.BoolVar(&flags.showHelp, "h", false, "Show help")
	flag.BoolVar(&flags.showHelp, "help", false, "Show help")
	flag.BoolVar(&flags.quiet, "quiet", false, "Suppress banners and non-essential output")
	flag.BoolVar(&flags.machine, "machine", false, "Minimal output: only print raw result")
	flag.Parse()
	flags.args = flag.Args()
	return &flags
}

// printHelp prints the CLI help message for mcp-client.
func printHelp() {
	fmt.Print(`mcp-client: Simple MCP client for openapi-to-mcp

Usage:
  mcp-client <server-command> [args...]

Flags:
  --quiet              Suppress banners and non-essential output
  --machine            Minimal output: only print raw result
  --help, -h           Show help

By default, output is human-friendly. Use --machine or --quiet for minimal/agent output.
`)
	os.Exit(0)
}

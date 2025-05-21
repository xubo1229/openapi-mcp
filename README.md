# <img src="https://raw.githubusercontent.com/jedisct1/openapi-mcp/main/.github/banner.png" alt="openapi-mcp" width="600"/>

# openapi-mcp

> **Expose any OpenAPI 3.x API as a robust, agent-friendly MCP tool server in seconds!**

[![Go Version](https://img.shields.io/badge/go-1.20%2B-blue)](https://golang.org/dl/)
[![Build Status](https://img.shields.io/github/actions/workflow/status/jedisct1/openapi-mcp/ci.yml?branch=main)](https://github.com/jedisct1/openapi-mcp/actions)
[![License](https://img.shields.io/github/license/jedisct1/openapi-mcp)](LICENSE)
[![GoDoc](https://pkg.go.dev/badge/github.com/jedisct1/openapi-mcp/pkg/openapi2mcp.svg)](https://pkg.go.dev/github.com/jedisct1/openapi-mcp/pkg/openapi2mcp)

---

**openapi-mcp** exposes all operations from an OpenAPI YAML or JSON file as [MCP](https://github.com/mark3labs/mcp-go) tools. It loads and validates your OpenAPI spec, generates MCP tools for each operation, and starts an MCP stdio or HTTP server. It also provides agent-friendly output, structured error handling, and robust automation features.

## 📚 Table of Contents
- [Features](#-features)
- [Quickstart](#-quickstart)
- [Authentication](#-authentication)
- [Usage Examples](#-usage-examples)
- [Agent Integration](#-agent-integration)
- [Documentation Generation](#-documentation-generation)
- [Contributing](#-contributing)
- [License](#-license)

## AI Agent-Friendly Features
openapi-mcp is designed from the ground up to be robust and easy to use for AI coding agents, LLMs, and automation:
- **Consistent, machine-readable output:** All tool results are always JSON or plain text, with explicit `OutputFormat` and `OutputType` fields. See [Output Types and Structure](#output-types-and-structure).
- **Structured error handling:** Validation and runtime errors are returned as structured JSON objects with error codes, messages, missing fields, and actionable suggestions.
- **Rich metadata and hints:** Tool responses include argument schemas, examples, usage, and next steps to help agents construct valid calls.
- **Agent-friendly documentation:** The `describe` tool returns a machine-readable schema for all tools, including argument types, constraints, and example calls. See [Agent-Friendly Documentation](#agent-friendly-documentation).
- **Streaming and partial output:** Tools can return partial results and `resume_token`s for chunked or long-running operations, enabling agents to resume or continue calls. See [Streaming/Partial output](#output-types-and-structure).
- **Explicit confirmation workflow:** Dangerous actions (PUT/POST/DELETE) return a structured `confirmation_required` response, so agents can handle confirmation in a standardized way. See [Safety and Confirmation](#safety-and-confirmation).
- **Minimal, non-verbose output:** Minimal, agent-friendly output is the default. Use `--extended` for human-friendly output.
- **Self-describing tools:** All tool schemas include type information, constraints, and examples, making it easy for agents to reason about valid arguments.
- **No redundant warnings:** Tool descriptions are concise and free of redundant warnings, relying on structured fields for all agent-relevant information.

## ✨ Features
- Loads and validates OpenAPI 3.x YAML/JSON files
- Exposes each OpenAPI operation as an MCP tool, with accurate input schemas
- Supports path, query, header, cookie, and body parameters
- Makes real HTTP calls to the described endpoints, with proper parameter substitution
- Adds tools for OpenAPI `info`, `externalDocs`, and a `describe` tool for agent-friendly documentation
- Pretty-prints all JSON output for readability
- Validates tool call arguments against the OpenAPI schema
- Supports API key, Bearer, Basic, and OAuth2 authentication (see [Authentication](#-authentication))
- Randomly selects a server URL from the OpenAPI `servers` list for each call
- **Safety:** For any tool that performs a PUT, POST, or DELETE, a structured confirmation is required before proceeding
- **Flexible configuration:** All environment variables can also be set as command-line flags
- **Interactive client:** The included MCP client supports readline, command history, and a user-friendly `mcp> ` prompt
- **Documentation generation:** Generate Markdown documentation for all tools, including argument schemas and example calls
- **Post-processing hooks:** Pipe generated schemas through any external command (e.g., jq, scripts)
- **CI/Agent-friendly:** Machine-readable output, robust exit codes, summary options, and agent-friendly schemas
- **Streaming/Partial output:** Tools can return partial results and resume tokens for long-running or chunked operations

## ⚡ Quickstart

### Prerequisites
- Go 1.20+
- An OpenAPI 3.x YAML or JSON file (e.g., `openapi.yaml`)

### Build
```sh
# Clone and build
$ git clone <repo-url>
$ cd openapi-mcp
$ make
```

This will place the command line tools in the `bin` directory:
- `bin/openapi-mcp`
- `bin/mcp-client`

### Run the MCP Server
```sh
$ API_KEY=your_api_key bin/openapi-mcp openapi.yaml
# or, using flags:
$ bin/openapi-mcp --api-key=your_api_key --base-url=https://api.example.com openapi.yaml
# To serve over HTTP:
$ bin/openapi-mcp --http=:8080 openapi.yaml
```

### Run the MCP Client
```sh
$ bin/mcp-client bin/openapi-mcp openapi.yaml
```

## 🔒 Authentication
openapi-mcp supports multiple authentication schemes as defined in OpenAPI 3.x:
- **API Key**: Use `--api-key` or `API_KEY` (header, query, or cookie, as defined in the spec)
- **Bearer Token / OAuth2**: Use `--bearer-token` or `BEARER_TOKEN` (sets `Authorization: Bearer ...`). For OAuth2, provide your access token directly; interactive flows are not yet supported.
- **Basic Auth**: Use `--basic-auth` or `BASIC_AUTH` (sets `Authorization: Basic ...`)

> **Note:** The correct authentication method is injected for each operation as defined in the OpenAPI spec (header, query, or cookie). If an operation requires OAuth2, you must provide a valid access token via `--bearer-token`/`BEARER_TOKEN`.

#### Examples
```sh
# Bearer Token / OAuth2
$ openapi-mcp --bearer-token "mytoken" openapi.yaml
# or
$ BEARER_TOKEN=mytoken openapi-mcp openapi.yaml

# API Key
$ openapi-mcp --api-key "mykey" openapi.yaml
# or
$ API_KEY=mykey openapi-mcp openapi.yaml

# Basic Auth
$ openapi-mcp --basic-auth "user:password" openapi.yaml
# or
$ BASIC_AUTH="user:password" openapi-mcp openapi.yaml
```

## 🛠️ Usage Examples

### Preview tools as JSON (dry run)
```sh
openapi-mcp --dry-run openapi.yaml
```
### Generate Markdown documentation
```sh
openapi-mcp --doc tools.md openapi.yaml
```
### Only include tools with the "admin" tag
```sh
openapi-mcp --tag admin openapi.yaml
```
### Print a summary for CI
```sh
openapi-mcp --summary --dry-run openapi.yaml
```
### Post-process schema with jq before writing docs
```sh
openapi-mcp --doc tools.md --post-hook-cmd 'jq . | tee /tmp/filtered.json' openapi.yaml
```
### Disable confirmation for dangerous actions
```sh
openapi-mcp --no-confirm-dangerous --doc tools.md openapi.yaml
```
### Use in CI pipeline (fail on error, machine-readable output)
```sh
openapi-mcp --dry-run --summary openapi.yaml > /tmp/schema.json
```

## Agent Integration

openapi-mcp is designed to make any OpenAPI-documented API accessible to AI coding agents, editors, and LLM-based tools (such as Cursor, Copilot, GPT-4, and others). By exposing each API operation as a machine-readable tool with rich schemas, examples, and structured output, agents can:
- Discover available operations and their arguments
- Validate and construct correct API calls
- Handle errors, confirmations, and streaming output
- Chain multiple API calls together in workflows

> **Tip:** For machine, agent, or CI usage, **do not use `--extended`**—the default output is minimal and agent-friendly. Use `--extended` only for human-friendly output (banners, help, etc.). Always use the `describe` tool to get up-to-date schemas and examples.

## 📝 Documentation Generation

Generate Markdown documentation for all tools, including argument schemas and example calls:
```sh
openapi-mcp --doc tools.md openapi.yaml
```

Pipe the generated tool schema JSON through any external command (e.g., jq, scripts) before output or documentation:
```sh
openapi-mcp --doc tools.md --post-hook-cmd 'jq . | tee /tmp/filtered.json' openapi.yaml
```

## 🙌 Contributing

Contributions, bug reports, and feature requests are welcome! Please open an issue or pull request on GitHub.

1. Fork the repo and create your branch from `main`
2. Make your changes (add tests for new features!)
3. Run `make test` to ensure all tests pass
4. Submit a pull request

## 📄 License

This project is licensed under the [MIT License](LICENSE).

## Library Usage: Go Module for MCP Servers

**openapi-mcp** is not just a CLI tool—it is also a Go module that can be imported by third-party applications to quickly add production-grade MCP server capabilities.

- **Import as a library:** Use `github.com/jedisct1/openapi-mcp/pkg/openapi2mcp` in your Go project.
- **Main entrypoints:**
  - `openapi2mcp.NewServer` — Create an MCP server from an OpenAPI spec.
  - `openapi2mcp.RegisterOpenAPITools` — Register OpenAPI operations as MCP tools on any server.
  - `openapi2mcp.ServeStdio` / `ServeHTTP` — Start the server in stdio or HTTP mode.
- **Production-ready:** All validation, error handling, and agent-friendly output are available via the Go API.
- **Extend or embed:** Add custom tools, post-processing, or integrate with your own authentication, logging, or orchestration.

See [GoDoc](https://pkg.go.dev/github.com/jedisct1/openapi-mcp/pkg/openapi2mcp) for full API documentation and code examples.

## Usage
- The OpenAPI spec path is a required argument: `openapi-mcp [flags] <openapi-spec-path>`
- The client can list tools and call them interactively, with command history and editing
- All tool input is validated against the OpenAPI schema
- The API key is sent as the correct header, as defined in the OpenAPI spec
- **Confirmation required:** For any tool that performs a PUT, POST, or DELETE, the first call returns a structured confirmation request. The action only proceeds if the client retries the call with a special argument (e.g., `{"__confirmed": true}`).
- **Agent-friendly documentation:** Use the `describe` tool to get a machine-readable schema for all tools, including argument types, constraints, and example calls.
- **Streaming/Partial output:** Tools can return partial results with a `partial: true` flag and a `resume_token` for continuation.

### CLI Flags and Environment Variables
- `--extended` — Enable extended (human-friendly) output (default is minimal/agent)
- `--api-key` / `API_KEY` — Your API key for authenticated endpoints (required for most APIs)
- `--bearer-token` / `BEARER_TOKEN` — Bearer token for Authorization header (preferred if both are set)
- `--basic-auth` / `BASIC_AUTH` — Basic auth (user:pass) for Authorization header
- `--base-url` / `OPENAPI_BASE_URL` — Override the base URL for HTTP calls (optional)
- `--http` — Serve MCP over HTTP instead of stdio
- `--include-desc-regex` / `INCLUDE_DESC_REGEX` — Only include APIs whose description matches this regex
- `--exclude-desc-regex` / `EXCLUDE_DESC_REGEX` — Exclude APIs whose description matches this regex
- `--dry-run` — Print the generated MCP tool schemas as JSON and exit
- `--doc` — Write Markdown/HTML documentation for all tools to this file (implies no server)
- `--doc-format` — Documentation format: markdown (default) or html
- `--post-hook-cmd` — Command to post-process the generated tool schema JSON (used in --dry-run or --doc mode)
- `--no-confirm-dangerous` — Disable confirmation for dangerous (PUT/POST/DELETE) actions

By default, `openapi-mcp` uses minimal, agent-friendly output. Use `--extended` for human-friendly output.

## Safety and Confirmation
For any tool that performs a PUT, POST, or DELETE, the first call returns a structured confirmation request:
```json
{
  "type": "confirmation_request",
  "confirmation_required": true,
  "message": "This action is irreversible. Proceed?",
  "action": "delete_resource"
}
```
The action only proceeds if the client retries the call with a special argument (e.g., `{"__confirmed": true}`).

## Output Types and Structure
- All tool results include `OutputFormat` ("structured" or "unstructured") and `OutputType` ("json", "text", etc.) fields.
- JSON responses are always pretty-printed and include a top-level `type` field (e.g., `{ "type": "api_response", ... }`).
- Partial/streaming results include `partial: true` and a `resume_token` for continuation.

## Agent-Friendly Documentation
- Use the `describe` tool to get a machine-readable schema for all tools, including argument types, constraints, output types, and example calls.
- Example call:
  ```json
  {
    "jsonrpc": "2.0",
    "id": 1,
    "method": "tools/call",
    "params": { "name": "describe", "arguments": {} }
  }
  ```
- The response will include a full schema for all tools, suitable for LLMs, code agents, and advanced clients.

## Documentation Generation
Generate Markdown documentation for all tools, including argument schemas and example calls:
```
openapi-mcp --doc tools.md openapi.yaml
```

Pipe the generated tool schema JSON through any external command (e.g., jq, scripts) before output or documentation:
```
openapi-mcp --doc tools.md --post-hook-cmd 'jq . | tee /tmp/filtered.json' openapi.yaml
```

## Usage Examples

### 1. Running openapi-mcp with a Sample OpenAPI Spec

```sh
openapi-mcp petstore.yaml --http-addr :8080 --summary
```
Expected output:
```
OpenAPI spec loaded and validated successfully.
Registered all OpenAPI operations as MCP tools.
Starting MCP server (HTTP) on :8080...
Total tools: 12
Tags:
  pets: 5
  store: 4
  users: 3
```

### 2. Using mcp-client to Interact with the Server

```sh
mcp-client './bin/openapi-mcp petstore.yaml --http-addr :8080'
```
Example session:
```
mcp> list
["listPets", "createPet", ...]
mcp> schema createPet
Schema for createPet:
{
  "type": "object",
  "properties": {
    "name": {"type": "string", "description": "Pet name"},
    "age": {"type": "integer", "description": "Pet age"}
  },
  "required": ["name"]
}
Example: call createPet {"name": "Fluffy", "age": 2}
mcp> call createPet {"name": "Fluffy", "age": 2}
HTTP POST /pets
Status: 201
Response:
{"id": 123, "name": "Fluffy", "age": 2}
```

### 3. Generating Documentation and Dry-Run Output

```sh
openapi-mcp petstore.yaml --doc petstore.md
openapi-mcp petstore.yaml --dry-run
```
Sample output for --dry-run:
```
[
  {
    "name": "listPets",
    "description": "List all pets",
    "tags": ["pets"],
    "inputSchema": { ... }
  },
  ...
]
```

### 4. Using openapi2mcp in Go Code

```go
package main

import (
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/jedisct1/openapi-mcp/pkg/openapi2mcp"
)

func main() {
	doc, err := openapi2mcp.LoadOpenAPISpec("petstore.yaml")
	if err != nil {
		panic(err)
	}
	srv := openapi2mcp.NewServer("petstore", doc.Info.Version, doc)
	if err := openapi2mcp.ServeHTTP(srv, ":8080"); err != nil {
		panic(err)
	}
}
```

See [GoDoc](https://pkg.go.dev/github.com/jedisct1/openapi-mcp/pkg/openapi2mcp) for more code examples and API details.

## Tutorial: Exposing OpenAPI APIs to AI Coding Agents

openapi-mcp is designed to make any OpenAPI-documented API accessible to AI coding agents, editors, and LLM-based tools (such as Cursor, Copilot, GPT-4, and others). By exposing each API operation as a machine-readable tool with rich schemas, examples, and structured output, agents can:
- Discover available operations and their arguments
- Validate and construct correct API calls
- Handle errors, confirmations, and streaming output
- Chain multiple API calls together in workflows

> **Note:** By default, openapi-mcp uses minimal, agent-friendly output. **Do not use `--extended` for machine/agent/CI usage.** Use `--extended` only for human-friendly output.

### Step-by-Step: Making Your API Agent-Accessible

1. **Obtain or author an OpenAPI 3.x YAML/JSON spec** for your API (e.g., `openapi.yaml`).
2. **Build and run openapi-mcp in stdio mode (recommended):**
   ```sh
   make
   bin/openapi-mcp openapi.yaml
   ```
   This starts an MCP server exposing all API operations as tools, with minimal, agent-friendly output, over stdio.
   - **To use HTTP mode instead:**
     ```sh
     bin/openapi-mcp openapi.yaml --http-addr :8080
     ```
3. **(Optional) Preview available tools:**
   ```sh
   bin/openapi-mcp openapi.yaml --dry-run --summary
   ```
4. **Connect your AI agent/editor:**
   - In Cursor, Copilot, or your preferred agent, configure a tool provider or plugin to connect to the running MCP server (stdio is usually the default and easiest option).
   - The agent can use the `describe` tool to discover all available tools, their schemas, and example calls:
     ```json
     { "jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": { "name": "describe", "arguments": {} } }
     ```
   - The agent can then list, call, and chain tools as needed, using the schemas and examples provided.

### Example: Using Cursor to Call an API

1. **Start the server in stdio mode (recommended):**
   ```sh
   bin/openapi-mcp openapi.yaml
   ```
   - Or, for HTTP mode:
     ```sh
     bin/openapi-mcp openapi.yaml --http-addr :8080
     ```
2. **In Cursor, add a tool provider** pointing to the MCP server (stdio is usually the default and easiest option).
3. **Discover tools:**
   - Use the agent's UI or API to call the `describe` tool and get a list of all available operations.
4. **Call a tool:**
   - The agent/editor will show argument schemas, examples, and usage for each tool.
   - Example: Call `createPet` with `{ "name": "Fluffy", "age": 2 }` and receive a structured JSON response.
5. **Handle confirmations and errors:**
   - For dangerous actions (PUT/POST/DELETE), the agent will receive a `confirmation_required` response and can prompt the user or retry with `{"__confirmed": true}`.
   - All errors are structured with actionable suggestions for agents.
6. **Chain tools:**
   - Agents can use the output of one tool as input to another, enabling complex workflows.

### Tips for Agent Integration
- Minimal, agent-friendly output is the default. **Do not use `--extended` for machine/agent/CI usage.** Use `--extended` only for human-friendly output.
- Always use the `describe` tool to get up-to-date schemas and examples.
- Handle `confirmation_required` and structured error responses programmatically.
- Use streaming/partial output and `resume_token` for long-running or chunked operations.
- For CI or automated agents, use `--dry-run`

## Authentication
openapi-mcp supports multiple authentication schemes as defined in OpenAPI 3.x:
- **API Key**: Use `--api-key` or `API_KEY` (header, query, or cookie, as defined in the spec)
- **Bearer Token / OAuth2**: Use `--bearer-token` or `BEARER_TOKEN` (sets `Authorization: Bearer ...`). For OAuth2, provide your access token directly; interactive flows are not yet supported.
- **Basic Auth**: Use `--basic-auth` or `BASIC_AUTH` (sets `Authorization: Basic ...`)

> **Note:** The correct authentication method is injected for each operation as defined in the OpenAPI spec (header, query, or cookie). If an operation requires OAuth2, you must provide a valid access token via `--bearer-token`/`BEARER_TOKEN`.

### Examples
#### Bearer Token / OAuth2
```
openapi-mcp --bearer-token "mytoken" openapi.yaml
# or
BEARER_TOKEN=mytoken openapi-mcp openapi.yaml
```

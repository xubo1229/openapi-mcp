## Commands

- `openapi-mcp [flags] <openapi-spec-path>`: Start the MCP server (stdio or HTTP)
- `openapi-mcp validate <openapi-spec-path>`: Validate the OpenAPI spec and report actionable errors
- `openapi-mcp lint <openapi-spec-path>`: Perform detailed OpenAPI linting with comprehensive suggestions
- `openapi-mcp filter <openapi-spec-path>`: Output a filtered list of operations as JSON, applying `--tag`, `--include-desc-regex`, and `--exclude-desc-regex` (no server)

## Usage

```sh
openapi-mcp [flags] <openapi-spec-path>
openapi-mcp validate <openapi-spec-path>
openapi-mcp lint <openapi-spec-path>
openapi-mcp filter [flags] <openapi-spec-path>
openapi-mcp --http=:8080 --mount /petstore:petstore.yaml --mount /books:books.yaml
```

## Examples

### Start MCP Server (stdio)
```sh
openapi-mcp api.yaml
```

### Start MCP Server over HTTP (single API)
```sh
openapi-mcp --http=:8080 api.yaml
```

### Start MCP Server over HTTP (multiple APIs)
```sh
openapi-mcp --http=:8080 --mount /petstore:petstore.yaml --mount /books:books.yaml
```
This will serve the Petstore API at `/petstore/sse`, `/petstore/message`, etc., and the Books API at `/books/sse`, `/books/message`, etc.

### Validate an OpenAPI Spec
```sh
openapi-mcp validate api.yaml
```

### Lint an OpenAPI Spec
```sh
openapi-mcp lint api.yaml
```

### Filter Operations by Tag and Description
```sh
openapi-mcp filter --tag=admin api.yaml
openapi-mcp filter --include-desc-regex=foo api.yaml
openapi-mcp filter --tag=admin --include-desc-regex=foo api.yaml
```

This will output a JSON array of operations matching the filters, including their name, description, tags, and input schema. 
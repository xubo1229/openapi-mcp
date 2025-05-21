# Binaries will be built into the ./bin directory
.PHONY: all mcp-client openapi-mcp clean

all: bin/mcp-client bin/openapi-mcp

bin/mcp-client: $(shell find pkg -type f -name '*.go') $(shell find cmd/mcp-client -type f -name '*.go')
	@mkdir -p bin
	go build -o bin/mcp-client ./cmd/mcp-client

bin/openapi-mcp: $(shell find pkg -type f -name '*.go') $(shell find cmd/openapi-mcp -type f -name '*.go')
	@mkdir -p bin
	go build -o bin/openapi-mcp ./cmd/openapi-mcp

test:
	go test ./...

clean:
	rm -f bin/mcp-client bin/openapi-mcp

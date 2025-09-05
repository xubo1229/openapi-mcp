# Binaries will be built into the ./bin directory
.PHONY: all clean test \
        bin/qihoo-mcp-linux-client bin/qihoo-mcp-mac-client \
        bin/qihoo-openapi-linux-mcp bin/qihoo-openapi-mac-mcp

all: \
    bin/qihoo-mcp-linux-client \
    bin/qihoo-mcp-mac-client \
    bin/qihoo-openapi-linux-mcp \
    bin/qihoo-openapi-mac-mcp

# ==== MCP CLIENT ====

# Linux 64-bit
bin/qihoo-mcp-linux-client: $(shell find pkg -type f -name '*.go') $(shell find cmd/mcp-client -type f -name '*.go')
	@mkdir -p bin
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o bin/qihoo-mcp-linux-client ./cmd/mcp-client

# macOS ARM64 (M1/M2)
bin/qihoo-mcp-mac-client: $(shell find pkg -type f -name '*.go') $(shell find cmd/mcp-client -type f -name '*.go')
	@mkdir -p bin
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -o bin/qihoo-mcp-mac-client ./cmd/mcp-client

# ==== OPENAPI MCP ====

# Linux 64-bit
bin/qihoo-openapi-linux-mcp: $(shell find pkg -type f -name '*.go') $(shell find cmd/openapi-mcp -type f -name '*.go')
	@mkdir -p bin
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o bin/qihoo-openapi-linux-mcp ./cmd/openapi-mcp

# macOS ARM64 (M1/M2)
bin/qihoo-openapi-mac-mcp: $(shell find pkg -type f -name '*.go') $(shell find cmd/openapi-mcp -type f -name '*.go')
	@mkdir -p bin
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -o bin/qihoo-openapi-mac-mcp ./cmd/openapi-mcp

test:
	go test ./...

clean:
	rm -f bin/qihoo-mcp-*-client bin/qihoo-openapi-*-mcp
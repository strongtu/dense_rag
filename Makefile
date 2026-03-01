MODULE     := dense-rag
BIN        := bin/$(MODULE)
MCP_BIN    := bin/$(MODULE)-mcp
GO         := go
GOFLAGS    :=

.PHONY: build build-mcp run run-mcp test lint clean

build:
	$(GO) build $(GOFLAGS) -o $(BIN) ./cmd/dense-rag

build-mcp:
	$(GO) build $(GOFLAGS) -o $(MCP_BIN) ./cmd/dense-rag-mcp

build-all: build build-mcp

run: build
	./$(BIN)

run-mcp: build-mcp
	./$(MCP_BIN)

test:
	$(GO) test ./... -v -count=1

lint:
	$(GO) vet ./...

clean:
	rm -rf bin/

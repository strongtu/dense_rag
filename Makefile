MODULE     := dense-rag
BIN        := bin/$(MODULE)
GO         := go
GOFLAGS    :=

.PHONY: build run test lint clean

build:
	$(GO) build $(GOFLAGS) -o $(BIN) ./cmd/dense-rag

run: build
	./$(BIN)

test:
	$(GO) test ./... -v -count=1

lint:
	$(GO) vet ./...

clean:
	rm -rf bin/

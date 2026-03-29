.PHONY: build test lint install clean

build:
	go build -o bin/linear-pp-cli ./cmd/linear-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/linear-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/linear-mcp ./cmd/linear-mcp

install-mcp:
	go install ./cmd/linear-mcp

build-all: build build-mcp

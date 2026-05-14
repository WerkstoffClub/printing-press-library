.PHONY: build test lint install clean

build:
	go build -o bin/songkick-pp-cli ./cmd/songkick-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/songkick-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/songkick-pp-mcp ./cmd/songkick-pp-mcp

install-mcp:
	go install ./cmd/songkick-pp-mcp

build-all: build build-mcp

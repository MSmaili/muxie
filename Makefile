.PHONY: build install test clean release

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -X 'github.com/MSmaili/hetki/cmd.Version=$(VERSION)' \
           -X 'github.com/MSmaili/hetki/cmd.GitCommit=$(COMMIT)' \
           -X 'github.com/MSmaili/hetki/cmd.BuildDate=$(DATE)'

build:
	go build -ldflags "$(LDFLAGS)" -o hetki .

install: build
	sudo mv hetki /usr/local/bin/hetki

test:
	go test -v ./...

clean:
	rm -f hetki
	rm -rf dist/

# Build for multiple platforms (for releases)
release:
	mkdir -p dist
	GOOS=darwin GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/hetki-darwin-amd64 .
	GOOS=darwin GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/hetki-darwin-arm64 .
	GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/hetki-linux-amd64 .
	GOOS=linux GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/hetki-linux-arm64 .
	@echo "Release binaries built in dist/"

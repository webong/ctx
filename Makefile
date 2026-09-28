.PHONY: install test test-shell test-go test-cross build-native

GO_CACHE ?= /tmp/ctx-go-build-cache
GO_MOD_CACHE ?= /tmp/ctx-go-mod-cache

install:
	./install.sh

build-native:
	GOCACHE=$(GO_CACHE) GOMODCACHE=$(GO_MOD_CACHE) go build -o /tmp/ctx-native ./cmd/ctx

test: test-shell test-go test-cross

test-shell:
	./tests/test.sh

test-go:
	GOCACHE=$(GO_CACHE) GOMODCACHE=$(GO_MOD_CACHE) go test ./...

test-cross:
	GOOS=windows GOARCH=amd64 GOCACHE=$(GO_CACHE) GOMODCACHE=$(GO_MOD_CACHE) go build -o /tmp/ctx-windows-amd64.exe ./cmd/ctx
	GOOS=windows GOARCH=arm64 GOCACHE=$(GO_CACHE) GOMODCACHE=$(GO_MOD_CACHE) go build -o /tmp/ctx-windows-arm64.exe ./cmd/ctx

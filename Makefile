.PHONY: install install-native test test-shell test-native-integration test-native-installer test-go test-cross build-native release

GO_CACHE ?= /tmp/ctx-go-build-cache
GO_MOD_CACHE ?= /tmp/ctx-go-mod-cache

install:
	./install.sh

install-native:
	./install-native.sh

build-native:
	GOCACHE=$(GO_CACHE) GOMODCACHE=$(GO_MOD_CACHE) go build -o /tmp/ctx-native ./cmd/ctx

release:
	sh ./scripts/build-release.sh "$(VERSION)" "$(or $(DIST),dist)"

test: test-shell test-native-integration test-native-installer test-go test-cross

test-shell:
	./tests/test.sh

test-native-integration:
	./tests/native_test.sh

test-native-installer:
	./tests/native_installer_test.sh

test-go:
	GOCACHE=$(GO_CACHE) GOMODCACHE=$(GO_MOD_CACHE) go test ./...

test-cross:
	GOOS=windows GOARCH=amd64 GOCACHE=$(GO_CACHE) GOMODCACHE=$(GO_MOD_CACHE) go build -o /tmp/ctx-windows-amd64.exe ./cmd/ctx
	GOOS=windows GOARCH=arm64 GOCACHE=$(GO_CACHE) GOMODCACHE=$(GO_MOD_CACHE) go build -o /tmp/ctx-windows-arm64.exe ./cmd/ctx

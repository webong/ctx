.PHONY: install install-native test test-native-integration test-native-installer test-go test-cross build-native release

GO_CACHE ?= /tmp/ctx-go-build-cache
GO_MOD_CACHE ?= /tmp/ctx-go-mod-cache

install:
	./install.sh

install-native:
	./install-native.sh

build-native:
	GOCACHE=$(GO_CACHE) GOMODCACHE=$(GO_MOD_CACHE) go build -o /tmp/ctx-native ./cmd/ctx
	for adapter in firefox chrome chromium safari; do GOCACHE=$(GO_CACHE) GOMODCACHE=$(GO_MOD_CACHE) go build -o /tmp/ctx-$$adapter-share ./adapters/$$adapter/native || exit; done

release:
	sh ./scripts/build-release.sh "$(VERSION)" "$(or $(DIST),dist)"

test: test-native-integration test-native-installer test-go test-cross

test-native-integration:
	./tests/native_test.sh

test-native-installer:
	./tests/native_installer_test.sh

test-go:
	GOCACHE=$(GO_CACHE) GOMODCACHE=$(GO_MOD_CACHE) go test ./...

test-cross:
	GOOS=windows GOARCH=amd64 GOCACHE=$(GO_CACHE) GOMODCACHE=$(GO_MOD_CACHE) go build -o /tmp/ctx-windows-amd64.exe ./cmd/ctx
	for adapter in firefox chrome chromium safari; do GOOS=windows GOARCH=amd64 GOCACHE=$(GO_CACHE) GOMODCACHE=$(GO_MOD_CACHE) go build -o /tmp/ctx-$$adapter-share-windows-amd64.exe ./adapters/$$adapter/native || exit; done
	GOOS=windows GOARCH=arm64 GOCACHE=$(GO_CACHE) GOMODCACHE=$(GO_MOD_CACHE) go build -o /tmp/ctx-windows-arm64.exe ./cmd/ctx
	for adapter in firefox chrome chromium safari; do GOOS=windows GOARCH=arm64 GOCACHE=$(GO_CACHE) GOMODCACHE=$(GO_MOD_CACHE) go build -o /tmp/ctx-$$adapter-share-windows-arm64.exe ./adapters/$$adapter/native || exit; done

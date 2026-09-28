#!/usr/bin/env sh
set -eu

VERSION=${1:-0.8.0-dev}
OUTPUT=${2:-dist}
ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
FIRST_PARTY_ADAPTERS='firefox chrome chromium safari kube aws gcloud postgres mysql'

case "$OUTPUT" in ''|/|.) printf 'ctx: unsafe release output directory: %s\n' "$OUTPUT" >&2; exit 2;; esac
if [ -e "$OUTPUT" ]; then
  printf 'ctx: release output already exists: %s\n' "$OUTPUT" >&2
  exit 1
fi
mkdir -p "$OUTPUT"
OUTPUT=$(CDPATH= cd -- "$OUTPUT" && pwd)

build_bundle() {
  os=$1
  arch=$2
  extension=$3
  staging=$(mktemp -d "$OUTPUT/.ctx-release.XXXXXX")
  bundle="$staging/ctx"
  mkdir -p "$bundle/bin" "$bundle/adapters"

  binary="$bundle/bin/ctx$extension"
  (cd "$ROOT" && CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" \
    go build -trimpath -ldflags "-s -w -X github.com/webong/ctx/internal/app.Version=${VERSION#v}" \
    -o "$binary" ./cmd/ctx)

  for engine in docker podman nerdctl; do
    if [ "$os" = windows ]; then
      cp "$ROOT/adapters/$engine/$engine.cmd" "$bundle/bin/$engine.cmd"
    else
      cp "$ROOT/adapters/$engine/$engine" "$bundle/bin/$engine"
      chmod +x "$bundle/bin/$engine"
    fi
  done
  for adapter in $FIRST_PARTY_ADAPTERS; do
    cp -R "$ROOT/adapters/$adapter" "$bundle/adapters/$adapter"
  done
  cp "$ROOT/LICENSE" "$ROOT/README.md" "$bundle/"

  if [ "$os" = windows ]; then
    (cd "$staging" && zip -qr "$OUTPUT/ctx-$os-$arch.zip" ctx)
  else
    tar -C "$staging" -czf "$OUTPUT/ctx-$os-$arch.tar.gz" ctx
  fi
  rm -rf "$staging"
}

build_bundle darwin amd64 ''
build_bundle darwin arm64 ''
build_bundle linux amd64 ''
build_bundle linux arm64 ''
build_bundle windows amd64 '.exe'
build_bundle windows arm64 '.exe'

#!/usr/bin/env sh
set -eu

VERSION=${1:-0.8.0-dev}
OUTPUT=${2:-dist}
ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
CATALOG_ADAPTERS='docker podman nerdctl apple rancher_desktop orbstack docker_desktop firefox zen floorp waterfox librewolf chrome chromium edge brave safari vivaldi opera whale arc comet dia atlas helium kube aws gcloud postgres mysql php claude_code codex'

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
  mkdir -p "$bundle/bin"

  binary="$bundle/bin/ctx$extension"
  (cd "$ROOT" && CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" \
    go build -trimpath -ldflags "-s -w -X github.com/webong/ctx/internal/app.Version=${VERSION#v}" \
    -o "$binary" ./cmd/ctx)
  cp "$ROOT/LICENSE" "$ROOT/README.md" "$bundle/"

  if [ "$os" = windows ]; then
    (cd "$staging" && zip -qr "$OUTPUT/ctx-$os-$arch.zip" ctx)
  else
    tar -C "$staging" -czf "$OUTPUT/ctx-$os-$arch.tar.gz" ctx
  fi

  mkdir -p "$bundle/adapters"
  for adapter in $CATALOG_ADAPTERS; do
    cp -R "$ROOT/adapters/$adapter" "$bundle/adapters/$adapter"
  done
  for adapter in firefox zen floorp waterfox librewolf chrome chromium edge brave safari vivaldi opera whale arc comet dia atlas helium; do
    (cd "$ROOT" && CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" \
      go build -trimpath -ldflags "-s -w" \
      -o "$bundle/adapters/$adapter/ctx-$adapter-share$extension" "./adapters/$adapter/native")
    rm -rf "$bundle/adapters/$adapter/native"
  done
  rm -rf "$bundle/adapters/chromium/engine"
  rm -rf "$bundle/adapters/firefox/engine"
  if [ "$os" = windows ]; then
    (cd "$staging" && zip -qr "$OUTPUT/ctx-adapters-$os-$arch.zip" ctx/adapters)
  else
    tar -C "$staging" -czf "$OUTPUT/ctx-adapters-$os-$arch.tar.gz" ctx/adapters
  fi
  rm -rf "$staging"
}

build_bundle darwin amd64 ''
build_bundle darwin arm64 ''
build_bundle linux amd64 ''
build_bundle linux arm64 ''
build_bundle windows amd64 '.exe'
build_bundle windows arm64 '.exe'

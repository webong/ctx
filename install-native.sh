#!/usr/bin/env sh
set -eu

DST_BIN=${CTX_BIN_DIR:-$HOME/.local/bin}
CONFIG_DIR=${CTX_HOME:-$HOME/.config/ctx}
VERSION=${CTX_VERSION:-latest}
FIRST_PARTY_ADAPTERS='docker podman nerdctl apple firefox chrome chromium safari kube aws gcloud postgres mysql'
ROOT=
if [ -f "$0" ]; then ROOT=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd); fi

temporary=$(mktemp -d "${TMPDIR:-/tmp}/ctx-native-install.XXXXXX")
trap 'rm -rf "$temporary"' EXIT HUP INT TERM
bundle="$temporary/ctx"

if [ -n "$ROOT" ] && [ -f "$ROOT/cmd/ctx/main.go" ]; then
  command -v go >/dev/null 2>&1 || { printf 'ctx: Go is required when installing from source\n' >&2; exit 1; }
  mkdir -p "$bundle/bin" "$bundle/adapters"
  (cd "$ROOT" && go build -o "$bundle/bin/ctx" ./cmd/ctx)
  for engine in docker podman nerdctl; do cp "$ROOT/adapters/$engine/$engine" "$bundle/bin/$engine"; done
  for adapter in $FIRST_PARTY_ADAPTERS; do cp -R "$ROOT/adapters/$adapter" "$bundle/adapters/$adapter"; done
else
  command -v curl >/dev/null 2>&1 || { printf 'ctx: curl is required for remote installation\n' >&2; exit 1; }
  os=$(uname -s)
  case "$os" in Darwin) os=darwin;; Linux) os=linux;; *) printf 'ctx: unsupported operating system: %s\n' "$os" >&2; exit 1;; esac
  arch=$(uname -m)
  case "$arch" in x86_64|amd64) arch=amd64;; arm64|aarch64) arch=arm64;; *) printf 'ctx: unsupported architecture: %s\n' "$arch" >&2; exit 1;; esac
  release=latest/download
  [ "$VERSION" = latest ] || release="download/$VERSION"
  archive="$temporary/ctx.tar.gz"
  asset="ctx-$os-$arch.tar.gz"
  release_base="https://github.com/webong/ctx/releases/$release"
  curl -fsSL "$release_base/$asset" -o "$archive"
  curl -fsSL "$release_base/checksums.txt" -o "$temporary/checksums.txt"
  expected=$(awk -v asset="$asset" '$2 == asset || $2 == "*" asset { print $1; exit }' "$temporary/checksums.txt")
  [ -n "$expected" ] || { printf 'ctx: release checksum is missing for %s\n' "$asset" >&2; exit 1; }
  if command -v shasum >/dev/null 2>&1; then actual=$(shasum -a 256 "$archive" | awk '{print $1}')
  elif command -v sha256sum >/dev/null 2>&1; then actual=$(sha256sum "$archive" | awk '{print $1}')
  else printf 'ctx: shasum or sha256sum is required to verify the release\n' >&2; exit 1
  fi
  [ "$actual" = "$expected" ] || { printf 'ctx: release checksum verification failed for %s\n' "$asset" >&2; exit 1; }
  tar -xzf "$archive" -C "$temporary"
fi

[ -x "$bundle/bin/ctx" ] || { printf 'ctx: native bundle is missing ctx\n' >&2; exit 1; }
for tool in ctx docker podman nerdctl; do
  target="$DST_BIN/$tool"
  if [ -e "$target" ] || [ -L "$target" ]; then
    if [ "$tool" = ctx ]; then
      "$target" version 2>/dev/null | grep -Eq '^ctx ' || { printf 'ctx: %s exists; choose another CTX_BIN_DIR\n' "$target" >&2; exit 1; }
    elif ! (dd if="$target" bs=256 count=1 2>/dev/null | grep -Eq 'ctx wrapper|ctx native'); then
      printf 'ctx: %s exists; choose another CTX_BIN_DIR\n' "$target" >&2; exit 1
    fi
  fi
done

mkdir -p "$DST_BIN" "$CONFIG_DIR/adapters"
cp "$bundle/bin/ctx" "$DST_BIN/ctx"
chmod +x "$DST_BIN/ctx"
for engine in docker podman nerdctl; do
  cp "$bundle/bin/$engine" "$DST_BIN/$engine"
  chmod +x "$DST_BIN/$engine"
done
for adapter in $FIRST_PARTY_ADAPTERS; do
  source_adapter="$bundle/adapters/$adapter"
  target_adapter="$CONFIG_DIR/adapters/$adapter"
  if [ -e "$target_adapter" ]; then
    if ! grep -Eq '^first_party[[:space:]]*=[[:space:]]*"true"' "$target_adapter/adapter.toml" 2>/dev/null; then
      printf 'ctx: refusing to replace non-first-party adapter %s\n' "$target_adapter" >&2
      exit 1
    fi
    rm -rf "$target_adapter"
  fi
  cp -R "$source_adapter" "$target_adapter"
done
for adapter in $FIRST_PARTY_ADAPTERS; do
  CTX_HOME="$CONFIG_DIR" "$DST_BIN/ctx" adapter trust "$adapter" >/dev/null
done

if [ ! -f "$CONFIG_DIR/config.toml" ]; then
  printf '%s\n' '# ctx native configuration' '# docker_default = "desktop-linux"' '# podman_default = "podman-machine-default"' '# nerdctl_default = "default"' > "$CONFIG_DIR/config.toml"
fi

printf 'Installed native ctx, container shims, and first-party adapters in %s\n' "$DST_BIN"
printf 'Config: %s/config.toml\n' "$CONFIG_DIR"
case ":$PATH:" in *":$DST_BIN:"*) ;; *) printf 'Add %s before Docker, Podman, and nerdctl on PATH.\n' "$DST_BIN";; esac

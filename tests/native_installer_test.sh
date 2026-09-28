#!/usr/bin/env sh
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
TEST_ROOT=$(mktemp -d)
trap 'rm -rf "$TEST_ROOT"' EXIT HUP INT TERM

export HOME="$TEST_ROOT/home"
export CTX_HOME="$TEST_ROOT/config"
export CTX_BIN_DIR="$TEST_ROOT/bin"
mkdir -p "$HOME"

"$ROOT/install-native.sh" --adapters docker,kube >/dev/null

test -x "$CTX_BIN_DIR/ctx"
test -x "$CTX_BIN_DIR/docker"
test ! -e "$CTX_BIN_DIR/podman"
test -d "$CTX_HOME/adapters/docker"
test -d "$CTX_HOME/adapters/kube"
test ! -e "$CTX_HOME/adapters/podman"
test -d "$CTX_HOME/catalog/adapters/podman"
CTX_HOME="$CTX_HOME" "$CTX_BIN_DIR/ctx" adapter available | grep -Eq '^podman[[:space:]]+container[[:space:]]+available'

CTX_HOME="$CTX_HOME" CTX_BIN_DIR="$CTX_BIN_DIR" "$CTX_BIN_DIR/ctx" adapter add podman >/dev/null
test -x "$CTX_BIN_DIR/podman"
test -d "$CTX_HOME/adapters/podman"

minimal_home="$TEST_ROOT/minimal-home"
minimal_config="$TEST_ROOT/minimal-config"
minimal_bin="$TEST_ROOT/minimal-bin"
mkdir -p "$minimal_home"
output=$(HOME="$minimal_home" CTX_HOME="$minimal_config" CTX_BIN_DIR="$minimal_bin" \
  "$ROOT/install-native.sh" </dev/null)
printf '%s\n' "$output" | grep -Fq 'No interactive terminal detected'
test -x "$minimal_bin/ctx"
test -d "$minimal_config/catalog/adapters/docker"
test ! -e "$minimal_config/adapters/docker"

printf 'ctx native installer tests passed\n'

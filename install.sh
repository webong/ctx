#!/usr/bin/env sh
set -eu

DST_BIN=${CTX_BIN_DIR:-$HOME/.local/bin}
CONFIG_DIR=${CTX_HOME:-$HOME/.config/ctx}
CORE_ENGINE_ADAPTERS='docker podman nerdctl'
FIRST_PARTY_ADAPTERS='firefox chrome chromium safari kube aws gcloud postgres mysql'

for tool in ctx $CORE_ENGINE_ADAPTERS; do
  target="$DST_BIN/$tool"
  if [ -e "$target" ] || [ -L "$target" ]; then
    if [ "$tool" = ctx ]; then marker='ctx command'; else marker='ctx wrapper|dctx shim'; fi
    if ! (dd if="$target" bs=256 count=1 2>/dev/null | grep -Eq "$marker"); then
      printf 'ctx: %s exists; choose another CTX_BIN_DIR\n' "$target" >&2
      exit 1
    fi
  fi
done

SRC_DIR=
if [ -f "$0" ]; then
  candidate=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
  if [ -f "$candidate/bin/ctx" ]; then
    source_complete=1
    for tool in $CORE_ENGINE_ADAPTERS; do
      [ -f "$candidate/adapters/$tool/$tool" ] || source_complete=0
    done
    [ "$source_complete" -eq 1 ] && SRC_DIR=$candidate
  fi
fi

if [ -z "$SRC_DIR" ]; then
  if ! command -v curl >/dev/null 2>&1; then
    printf 'ctx: curl is required for streamed installation\n' >&2
    exit 1
  fi
  FETCH_DIR=$(mktemp -d)
  trap 'rm -rf "$FETCH_DIR"' 0
  SOURCE_BASE=${CTX_SOURCE_BASE:-https://raw.githubusercontent.com/webong/ctx/${CTX_REF:-main}}
  mkdir -p "$FETCH_DIR/bin" "$FETCH_DIR/adapters"
  curl -fsSL "$SOURCE_BASE/bin/ctx" -o "$FETCH_DIR/bin/ctx"
  [ -s "$FETCH_DIR/bin/ctx" ] || { printf 'ctx: empty download for ctx\n' >&2; exit 1; }
  sh -n "$FETCH_DIR/bin/ctx"
  for tool in $CORE_ENGINE_ADAPTERS; do
    mkdir -p "$FETCH_DIR/adapters/$tool"
    curl -fsSL "$SOURCE_BASE/adapters/$tool/$tool" -o "$FETCH_DIR/adapters/$tool/$tool"
    if [ ! -s "$FETCH_DIR/adapters/$tool/$tool" ]; then
      printf 'ctx: empty download for %s adapter\n' "$tool" >&2
      exit 1
    fi
    sh -n "$FETCH_DIR/adapters/$tool/$tool"
    chmod +x "$FETCH_DIR/adapters/$tool/$tool"
  done
  for adapter in $FIRST_PARTY_ADAPTERS; do
    mkdir -p "$FETCH_DIR/adapters/$adapter"
    curl -fsSL "$SOURCE_BASE/adapters/$adapter/adapter.toml" -o "$FETCH_DIR/adapters/$adapter/adapter.toml"
    executable=$(awk -F '"' '/^executable[[:space:]]*=/{ print $2; exit }' "$FETCH_DIR/adapters/$adapter/adapter.toml")
    [ -n "$executable" ] || { printf 'ctx: invalid first-party adapter manifest for %s\n' "$adapter" >&2; exit 1; }
    curl -fsSL "$SOURCE_BASE/adapters/$adapter/$executable" -o "$FETCH_DIR/adapters/$adapter/$executable"
    chmod +x "$FETCH_DIR/adapters/$adapter/$executable"
  done
  SRC_DIR=$FETCH_DIR
fi

mkdir -p "$DST_BIN" "$CONFIG_DIR" "$CONFIG_DIR/adapters"
cp "$SRC_DIR/bin/ctx" "$DST_BIN/ctx"
chmod +x "$DST_BIN/ctx"
for tool in $CORE_ENGINE_ADAPTERS; do
  cp "$SRC_DIR/adapters/$tool/$tool" "$DST_BIN/$tool"
  chmod +x "$DST_BIN/$tool"
done

for adapter in $FIRST_PARTY_ADAPTERS; do
  source_adapter="$SRC_DIR/adapters/$adapter"
  target_adapter="$CONFIG_DIR/adapters/$adapter"
  [ -f "$source_adapter/adapter.toml" ] || { printf 'ctx: bundled adapter %s is missing\n' "$adapter" >&2; exit 1; }
  if [ -e "$target_adapter" ]; then
    if ! grep -Eq '^first_party[[:space:]]*=[[:space:]]*"true"' "$target_adapter/adapter.toml" 2>/dev/null; then
      printf 'ctx: refusing to replace non-first-party adapter %s\n' "$target_adapter" >&2
      exit 1
    fi
    rm -rf "$target_adapter"
  fi
  mkdir -p "$target_adapter"
  cp -R "$source_adapter/." "$target_adapter/"
done

for adapter in $FIRST_PARTY_ADAPTERS; do
  CTX_HOME="$CONFIG_DIR" "$DST_BIN/ctx" adapter trust "$adapter" >/dev/null
done

if [ ! -f "$CONFIG_DIR/config.toml" ]; then
  cat > "$CONFIG_DIR/config.toml" <<'TOML'
# Optional fallbacks when a project has no .ctx choice.
# docker_default = "my-docker-context"
# podman_default = "my-podman-connection"
# nerdctl_default = "default"

# Optional named bundle:
# [profiles."client-a"]
# docker = "my-docker-context"
# kube_context = "client-a-dev"
# kube_namespace = "payments"
# aws_profile = "client-a"
# gcloud_configuration = "client-a"
# browser = "firefox:client-a"
# postgres_service = "client-a-dev"
# mysql_login_path = "client-a"
# shell_path = "/opt/client-a/bin"
# [profiles."client-a".env]
# APP_ENV = "development"

# Optional project mapping:
# [projects."/absolute/path/to/project"]
# docker = "my-docker-context"
# podman = "my-podman-connection"
# nerdctl = "default"
TOML
fi

printf 'Installed ctx, docker, podman, and nerdctl wrappers in %s\n' "$DST_BIN"
case ":$PATH:" in
  *":$DST_BIN:"*) ;;
  *) printf 'Add this directory before Docker, Podman, and nerdctl on PATH: export PATH="%s:$PATH"\n' "$DST_BIN" ;;
esac
printf 'Config: %s/config.toml\n' "$CONFIG_DIR"
printf 'Adapters: %s/adapters\n' "$CONFIG_DIR"
for rc in "$HOME/.zshrc" "$HOME/.bashrc"; do
  if [ -f "$rc" ] && grep -q 'dctx hook' "$rc"; then
    printf 'Old dctx shell hook found in %s; remove that line so it cannot override ctx.\n' "$rc"
  fi
done

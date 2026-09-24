#!/usr/bin/env sh
set -eu

SRC_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
DST_BIN=${CTX_BIN_DIR:-$HOME/.local/bin}
CONFIG_DIR=${CTX_HOME:-$HOME/.config/ctx}

for tool in ctx docker podman; do
  target="$DST_BIN/$tool"
  if [ -e "$target" ] || [ -L "$target" ]; then
    if [ "$tool" = ctx ]; then marker='ctx command'; else marker='ctx wrapper|dctx shim'; fi
    if ! (dd if="$target" bs=256 count=1 2>/dev/null | grep -Eq "$marker"); then
      printf 'ctx: %s exists; choose another CTX_BIN_DIR\n' "$target" >&2
      exit 1
    fi
  fi
done

mkdir -p "$DST_BIN" "$CONFIG_DIR"
for tool in ctx docker podman; do
  cp "$SRC_DIR/bin/$tool" "$DST_BIN/$tool"
  chmod +x "$DST_BIN/$tool"
done

if [ ! -f "$CONFIG_DIR/config.toml" ]; then
  cat > "$CONFIG_DIR/config.toml" <<'TOML'
# Optional fallbacks when a project has no .ctx choice.
# docker_default = "my-docker-context"
# podman_default = "my-podman-connection"

# Optional project mapping:
# [projects."/absolute/path/to/project"]
# docker = "my-docker-context"
# podman = "my-podman-connection"
TOML
fi

printf 'Installed ctx, docker, and podman wrappers in %s\n' "$DST_BIN"
case ":$PATH:" in
  *":$DST_BIN:"*) ;;
  *) printf 'Add this directory before Docker and Podman on PATH: export PATH="%s:$PATH"\n' "$DST_BIN" ;;
esac
printf 'Config: %s/config.toml\n' "$CONFIG_DIR"

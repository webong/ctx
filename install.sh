#!/usr/bin/env sh
set -eu

SRC_DIR="$(cd "$(dirname "$0")" && pwd)"
DST_BIN="$HOME/.local/bin"
CONFIG_DIR="$HOME/.config/dctx"

echo "→ dctx install $SRC_DIR → $DST_BIN"

mkdir -p "$DST_BIN" "$CONFIG_DIR"

# 1) install binaries
cp "$SRC_DIR/bin/dctx" "$DST_BIN/dctx"
cp "$SRC_DIR/bin/docker" "$DST_BIN/docker"
chmod +x "$DST_BIN/dctx" "$DST_BIN/docker"
echo "✓ installed $DST_BIN/dctx and $DST_BIN/docker (shim)"

# 2) ensure PATH has ~/.local/bin first
for rc in "$HOME/.zshrc" "$HOME/.bashrc"; do
  if [ -f "$rc" ] && ! grep -q '.local/bin' "$rc"; then
    printf '\nexport PATH="$HOME/.local/bin:$PATH"\n' >> "$rc"
    echo "✓ added PATH to $rc"
  fi
done

# 3) shell hooks
add_hook() {
  shell="$1"; rc="$2"; line="$3"
  if [ -f "$rc" ]; then
    if ! grep -q "dctx hook $shell" "$rc"; then
      printf '\n# dctx per-project hook\n%s\n' "$line" >> "$rc"
      echo "✓ hook added to $rc"
    else
      echo "• hook already in $rc"
    fi
  fi
}
add_hook "zsh" "$HOME/.zshrc" 'eval "$(dctx hook zsh)"'
add_hook "bash" "$HOME/.bashrc" 'eval "$(dctx hook bash)"'
if [ -d "$HOME/.config/fish" ]; then
  add_hook "fish" "$HOME/.config/fish/config.fish" 'dctx hook fish | source'
fi

# 4) config
if [ ! -f "$CONFIG_DIR/config.toml" ]; then
  cat > "$CONFIG_DIR/config.toml" <<'TOML'
# dctx config — shareable across machines
# default fallback when no .docker-context and only one daemon probe fails
default = "orbstack"

[projects]
# "/Users/webong/Workspace/Projects/AllAccess/allfans" = "orbstack"
# "/Users/webong/Workspace/Projects/AllAccess/backass" = "desktop-linux"
TOML
  echo "✓ created $CONFIG_DIR/config.toml"
else
  echo "• config exists $CONFIG_DIR/config.toml"
fi

# 5) de-duplicate old zsh docker() function if present (we now use shim)
if grep -q "Docker context auto-switch" "$HOME/.zshrc" 2>/dev/null; then
  echo "→ removing legacy docker() function from ~/.zshrc (now handled by shim)"
  # backup
  cp "$HOME/.zshrc" "$HOME/.zshrc.bak.$(date +%s)"
  # remove block from "# Docker context" to next "^}" inclusive
  # use awk to filter
  awk 'BEGIN{p=1} /# Docker context auto-switch/{p=0} p; /^\}.*# helper/ {if(p==0){p=1; next}} /__docker_context_chpwd/ {if(p==0) next} ' "$HOME/.zshrc.bak."* 2>/dev/null | head -1 >/dev/null || true
  # simpler: just inform user to manually remove
  echo "  backup at ~/.zshrc.bak.* — please remove old docker() + __docker_context_chpwd + dctx() blocks if present"
fi

echo ""
echo "Done. Restart shell or run: source ~/.zshrc"
echo "Try: dctx status; dctx ls; dctx set orbstack"
echo "Share: git add .docker-context"

# dctx — Docker Context Auto-Switcher

Universal, shareable per-project Docker context switcher for OrbStack ↔ Docker Desktop.

Works everywhere: `zsh`/`bash`/`fish`, VS Code, JetBrains, `docker compose`, CI — because it’s a **binary shim** on `PATH`, not a shell alias.

## Why

`~/.docker/config.json:10` `currentContext` is global. You want:
- `~/Workspace/Projects/AllAccess/allfans` → `orbstack`
- `~/Workspace/playground/desktop-test` → `desktop-linux`
- `cd` elsewhere → fallback to whichever daemon is running (last opened app wins)

`dctx` gives you that with zero per-terminal `docker context use`.

## Install

```bash
# curl (shareable)
curl -fsSL https://raw.githubusercontent.com/<you>/dctx/main/install.sh | bash

# or manual
git clone https://github.com/<you>/dctx ~/.local/share/dctx
~/.local/share/dctx/install.sh
```

`install.sh` does:
1. copies `bin/docker` shim → `~/.local/bin/docker` (before `/opt/homebrew/bin`/`~/.orbstack/bin` on PATH)
2. copies `bin/dctx` → `~/.local/bin/dctx`
3. adds `eval "$(dctx hook zsh)"` to `~/.zshrc` (and bash/fish equivalents)
4. creates `~/.config/dctx/config.toml`

## Usage

```bash
dctx ls                          # docker context ls + project map
dctx status                      # show what ctx would be used for $PWD
dctx set orbstack                # echo orbstack > ./.docker-context (per-project)
dctx set desktop-linux --global  # set fallback default
dctx set default --clear         # rm ./.docker-context

# per-project file (checked walking up to $HOME)
cat .docker-context
# orbstack

# any docker command now auto-routes:
docker ps
docker compose up -d
docker info --format '{{.Name}}'  # → orbstack or docker-desktop
```

## How it works

Priority per `docker` invocation (`bin/docker` shim):

1. `DOCKER_CONTEXT` / `DOCKER_HOST` env → respect
2. explicit `--context` / `--host` / `-H` flag → respect
3. `.docker-context` file walk-up from `$PWD` → `orbstack|desktop-linux|default`
4. `~/.config/dctx/config.toml` `[projects]` map
5. probe sockets: `~/.orbstack/run/docker.sock` vs `~/.docker/run/docker.sock` (+ `docker --context X info` check + mtime tie-break)
6. fallback → real `docker` with no `--context`

Shell hook (`shell/hook.zsh`) only exports `DOCKER_CONTEXT` for prompt/plugins; shim is the source of truth so GUI apps work too.

## Share across team (local-only)

`.docker-context` is **gitignored** — local per-dev, not committed. `dctx set` auto-adds it to `.gitignore`:

```bash
echo "orbstack" > .docker-context   # local only, ignored
# already in .gitignore: .docker-context
```

Share the *tool* not the choice:

```bash
git clone https://github.com/<you>/dctx ~/.local/share/dctx && ~/.local/share/dctx/install.sh
```

Teammate picks their own: `dctx set desktop-linux` (or `orbstack`) locally. Fallback still probes sockets if no file.

## Uninstall

```bash
dctx uninstall
# or
rm ~/.local/bin/docker ~/.local/bin/dctx
# remove eval line from ~/.zshrc
```

## License

MIT

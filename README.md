# ctx

ctx chooses container engine connections per project. It wraps the Docker and Podman commands, so the same directory can select a Docker context, a Podman connection, or both. It does not change either tool's global default.

## Install

You need curl and at least one of the Docker or Podman CLIs. The installer downloads the three commands when run as a stream:

~~~sh
curl -fsSL https://raw.githubusercontent.com/webong/ctx/main/install.sh | sh
~~~

To install from a local checkout instead, use Git:

~~~sh
git clone https://github.com/webong/ctx.git "$HOME/.local/share/ctx"
"$HOME/.local/share/ctx/install.sh"
~~~

Put $HOME/.local/bin before the real Docker and Podman commands on your PATH. For example, add this to ~/.zshrc or ~/.bashrc:

~~~sh
export PATH="$HOME/.local/bin:$PATH"
~~~

Open a new shell and check `command -v ctx`, `command -v docker`, and `command -v podman`. The installer leaves an existing config in place and refuses to replace an unrelated command. Set CTX_BIN_DIR and CTX_HOME to change installation locations.

If you previously installed dctx, remove any `eval "$(dctx hook zsh)"` or `eval "$(dctx hook bash)"` line from your shell startup file. That older hook can export DOCKER_CONTEXT and override .ctx. Set your project choices again with `ctx set`; the old .docker-context and dctx config are not read by ctx.

## Choose connections

Run these in a project directory:

~~~sh
ctx ls docker
ctx ls podman
ctx set docker my-docker-context
ctx set podman my-podman-connection
ctx status
~~~

This creates a local .ctx file:

~~~toml
docker = "my-docker-context"
podman = "my-podman-connection"
~~~

ctx set verifies that the named Docker context or Podman system connection exists. In a Git repository, it excludes .ctx through the local .git/info/exclude file. The choice stays on your machine, without modifying the project's tracked .gitignore. Use `ctx clear docker`, `ctx clear podman`, or `ctx clear` to remove choices.

~~~sh
docker ps
podman ps
docker build -f Containerfile .
podman build -f Containerfile .
~~~

The build commands use the selected connection. A Containerfile describes the image build; it does not select Docker or Podman.

## Build caches and transfers

Docker builders keep their local caches separate. Use `ctx build` to build through the
project's resolved Docker context while importing and exporting a BuildKit registry
cache. The registry reference must be writable by the caller. Give branches or
concurrent builders distinct cache references so they do not overwrite each other.

~~~sh
ctx build --cache-ref ghcr.io/acme/api:buildcache -- --tag ghcr.io/acme/api:dev .
~~~

Images also belong to the Docker daemon selected by a context. Copy a tagged image
between contexts with a registry, or use `--tar` for a temporary local archive when
both contexts are reachable from the same machine:

~~~sh
ctx image sync orbstack desktop-linux ghcr.io/acme/api:dev
ctx image sync --tar orbstack desktop-linux acme/api:dev
~~~

Use `ctx image copy` to bridge Docker and Podman through a temporary Docker archive.
Endpoints must be written as `docker:<context>` or `podman:<connection>`. The archive
passes through the machine running ctx, so both endpoints must be reachable there.

~~~sh
ctx image copy docker:orbstack podman:podman-machine-default acme/api:dev
ctx image copy podman:podman-machine-default docker:orbstack acme/api:dev
~~~

Named volumes are likewise private to each daemon. `ctx volume export` writes a tar
archive to standard output, and `ctx volume import` reads one from standard input.
Import refuses an existing target volume so it cannot silently merge data. Stop or
quiesce databases before export; this is a migration/backup tool, not live shared
storage.

~~~sh
ctx volume export orbstack postgres-data > postgres-data.tar
ctx volume import desktop-linux postgres-data < postgres-data.tar
~~~

`ctx volume copy` applies the same export/import approach between Docker and Podman.
It creates the target volume and refuses to merge into an existing one. It is still a
point-in-time migration, never live shared storage.

~~~sh
ctx volume copy docker:orbstack podman:podman-machine-default postgres-data postgres-data
ctx volume copy podman:podman-machine-default docker:orbstack postgres-data postgres-data
~~~

For another machine, stream the archive through a secure transport and run the
import command there. These commands use a short-lived Alpine container; set
`CTX_VOLUME_IMAGE` if your environment requires a different approved image.

## Optional central config

You can set fallbacks or map project paths in $HOME/.config/ctx/config.toml:

~~~toml
docker_default = "my-docker-context"
podman_default = "my-podman-connection"

[projects."/absolute/path/to/project"]
docker = "another-docker-context"
podman = "another-podman-connection"
~~~

Mappings apply to the named directory and its descendants. You can also set a fallback with `ctx set docker NAME --global` or `ctx set podman NAME --global`. If no Podman connection is selected, Podman keeps its own default behavior, including local operation on Linux.

## Selection order

1. Explicit command flags or connection environment variables take priority.
2. The nearest .ctx entry or central project mapping is used.
3. Docker can detect a running OrbStack or Docker Desktop daemon on macOS. Podman has no automatic daemon selection.
4. The optional per-tool fallback is used.
5. The unmodified CLI chooses its own default.

`docker context ...`, `podman system connection ...`, and `podman machine ...` pass through to the real CLI. The wrappers affect commands launched through them; other applications may use their own connection settings.

Podman connection selection uses its [--connection option](https://docs.podman.io/en/latest/markdown/podman.1.html). `podman build` accepts [Containerfiles](https://docs.podman.io/en/stable/markdown/podman-build.1.html).

## Remove

Remove the installed ctx, docker, and podman wrappers from $HOME/.local/bin after checking they belong to ctx. Your config is in $HOME/.config/ctx. If you added the PATH line solely for ctx, remove that line from your shell startup file.

## License

MIT

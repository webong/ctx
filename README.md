# ctx

ctx chooses container engine connections per project. It wraps Docker, Podman, and nerdctl, so the same directory can select a Docker context, a Podman connection, and a containerd namespace. It does not change any tool's global default. Apple Container is supported as an explicit transfer endpoint.

## Install

You need curl and at least one of the Docker, Podman, or nerdctl CLIs. The installer downloads ctx and its three wrappers when run as a stream:

~~~sh
curl -fsSL https://raw.githubusercontent.com/webong/ctx/main/install.sh | sh
~~~

To install from a local checkout instead, use Git:

~~~sh
git clone https://github.com/webong/ctx.git "$HOME/.local/share/ctx"
"$HOME/.local/share/ctx/install.sh"
~~~

Put $HOME/.local/bin before the real Docker, Podman, and nerdctl commands on your PATH. For example, add this to ~/.zshrc or ~/.bashrc:

~~~sh
export PATH="$HOME/.local/bin:$PATH"
~~~

Open a new shell and check `command -v ctx`, `command -v docker`, `command -v podman`, and `command -v nerdctl`. The installer leaves an existing config in place and refuses to replace an unrelated command. Set CTX_BIN_DIR and CTX_HOME to change installation locations.

If you previously installed dctx, remove any `eval "$(dctx hook zsh)"` or `eval "$(dctx hook bash)"` line from your shell startup file. That older hook can export DOCKER_CONTEXT and override .ctx. Set your project choices again with `ctx set`; the old .docker-context and dctx config are not read by ctx.

## Choose connections

Run these in a project directory:

~~~sh
ctx ls docker
ctx ls podman
ctx ls nerdctl
ctx set docker my-docker-context
ctx set podman my-podman-connection
ctx set nerdctl k8s.io
ctx status
~~~

This creates a local .ctx file:

~~~toml
docker = "my-docker-context"
podman = "my-podman-connection"
nerdctl = "k8s.io"
~~~

ctx set verifies that the named Docker context, Podman system connection, or nerdctl namespace exists. In a Git repository, it excludes .ctx through the local .git/info/exclude file. The choice stays on your machine, without modifying the project's tracked .gitignore. Use `ctx clear TOOL` or `ctx clear` to remove choices.

~~~sh
docker ps
podman ps
nerdctl ps
docker build -f Containerfile .
podman build -f Containerfile .
nerdctl build -f Containerfile .
~~~

The build commands use the selected connection or namespace. A Containerfile describes the image build; it does not select the engine.

## Build caches and transfers

Docker builders keep their local caches separate. Use `ctx build` to build through the
project's resolved Docker context while importing and exporting a BuildKit registry
cache. The registry reference must be writable by the caller. Give branches or
concurrent builders distinct cache references so they do not overwrite each other.

~~~sh
ctx build docker --cache-ref ghcr.io/acme/api:docker-cache -- --tag ghcr.io/acme/api:dev .
ctx build podman --cache-ref ghcr.io/acme/api:podman-cache -- --tag ghcr.io/acme/api:dev .
ctx build nerdctl --cache-ref ghcr.io/acme/api:nerdctl-cache -- --tag ghcr.io/acme/api:dev .
~~~

Omitting the engine keeps the original Docker behavior. Use distinct cache references:
Docker BuildKit, Podman/Buildah, and nerdctl BuildKit do not share one portable cache
format even though each can store cache data in a registry.

Images also belong to the Docker daemon selected by a context. Copy a tagged image
between contexts with a registry, or use `--tar` for a temporary local archive when
both contexts are reachable from the same machine:

~~~sh
ctx image sync orbstack desktop-linux ghcr.io/acme/api:dev
ctx image sync --tar orbstack desktop-linux acme/api:dev
~~~

Use `ctx image copy` to bridge Docker, Podman, nerdctl, and Apple Container through a
temporary image archive. Endpoints use `docker:<context>`, `podman:<connection>`,
`nerdctl:<namespace>`, or `apple:local`. The archive passes through the machine
running ctx, so both endpoints must be reachable there.

~~~sh
ctx image copy docker:orbstack podman:podman-machine-default acme/api:dev
ctx image copy podman:podman-machine-default docker:orbstack acme/api:dev
ctx image copy docker:orbstack nerdctl:k8s.io acme/api:dev
ctx image copy nerdctl:default apple:local acme/api:dev
~~~

For safety, imports into Apple Container require a version newer than 1.3.0. Older
versions have known image-loading vulnerabilities; update Apple Container before
using it as a copy target. See [GHSA-r3h2-rgqf-9hv9](https://github.com/apple/containerization/security/advisories/GHSA-r3h2-rgqf-9hv9).

Named volumes are likewise private to each daemon. `ctx volume export` writes a tar
archive to standard output, and `ctx volume import` reads one from standard input.
Import refuses an existing target volume so it cannot silently merge data. Stop or
quiesce databases before export; this is a migration/backup tool, not live shared
storage.

~~~sh
ctx volume export orbstack postgres-data > postgres-data.tar
ctx volume import desktop-linux postgres-data < postgres-data.tar
~~~

`ctx volume copy` applies the same export/import approach between Docker, Podman,
nerdctl, and Apple Container.
It creates the target volume and refuses to merge into an existing one. It is still a
point-in-time migration, never live shared storage.

~~~sh
ctx volume copy docker:orbstack podman:podman-machine-default postgres-data postgres-data
ctx volume copy podman:podman-machine-default docker:orbstack postgres-data postgres-data
ctx volume copy docker:orbstack nerdctl:default postgres-data postgres-data
ctx volume copy nerdctl:default apple:local postgres-data postgres-data
~~~

For another machine, stream the archive through a secure transport and run the
import command there. These commands use a short-lived Alpine container; set
`CTX_VOLUME_IMAGE` if your environment requires a different approved image.

## Optional central config

You can set fallbacks or map project paths in $HOME/.config/ctx/config.toml:

~~~toml
docker_default = "my-docker-context"
podman_default = "my-podman-connection"
nerdctl_default = "default"

[projects."/absolute/path/to/project"]
docker = "another-docker-context"
podman = "another-podman-connection"
nerdctl = "k8s.io"
~~~

Mappings apply to the named directory and its descendants. Set a fallback with `ctx set TOOL NAME --global`. If no Podman connection or nerdctl namespace is selected, that CLI keeps its own default behavior.

## Selection order

1. Explicit command flags or connection environment variables take priority.
2. The nearest .ctx entry or central project mapping is used.
3. Docker can detect a running OrbStack or Docker Desktop daemon on macOS. Podman and nerdctl have no automatic endpoint selection.
4. The optional per-tool fallback is used.
5. The unmodified CLI chooses its own default.

`docker context ...`, `podman system connection ...`, `podman machine ...`, and `nerdctl namespace ...` pass through to the real CLI. The wrappers affect commands launched through them; other applications may use their own connection settings.

Podman connection selection uses its [--connection option](https://docs.podman.io/en/latest/markdown/podman.1.html). nerdctl selection uses [containerd namespaces](https://github.com/containerd/nerdctl/blob/main/docs/command-reference.md#namespace-management). Apple Container transfer support follows its [image and volume commands](https://github.com/apple/container/blob/main/docs/command-reference.md).

## Remove

Remove the installed ctx, docker, podman, and nerdctl wrappers from $HOME/.local/bin after checking they belong to ctx. Your config is in $HOME/.config/ctx. If you added the PATH line solely for ctx, remove that line from your shell startup file.

## License

MIT

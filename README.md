# ctx

ctx chooses container engine connections per project. It wraps the Docker and Podman commands, so the same directory can select a Docker context, a Podman connection, or both. It does not change either tool's global default.

## Install

You need Git and at least one of the Docker or Podman CLIs.

~~~sh
git clone https://github.com/webong/ctx.git "$HOME/.local/share/ctx"
"$HOME/.local/share/ctx/install.sh"
~~~

Put $HOME/.local/bin before the real Docker and Podman commands on your PATH. For example, add this to ~/.zshrc or ~/.bashrc:

~~~sh
export PATH="$HOME/.local/bin:$PATH"
~~~

Open a new shell and check `command -v ctx`, `command -v docker`, and `command -v podman`. The installer leaves an existing config in place and refuses to replace an unrelated command. Set CTX_BIN_DIR and CTX_HOME to change installation locations.

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

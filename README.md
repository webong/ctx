# ctx

ctx chooses tool contexts per project. Its core owns shell environments, browser
profiles, and the container family: Docker, Podman, nerdctl/containerd, plus
Apple Container transfer endpoints. Kubernetes, AWS, gcloud, PostgreSQL, and
MySQL are first-party adapter packages developed in this repository and installed
with ctx. Named profiles bundle all of these choices without changing a tool's
global default. External adapters can add selectors without changing ctx itself.

## Install

You need curl and at least one of the Docker, Podman, or nerdctl CLIs. kubectl and
the AWS CLI, gcloud, database clients, and browsers are optional and only needed
for their adapters. The installer downloads ctx, its three container wrappers,
and the bundled first-party adapters when run as a stream:

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

Kubernetes and AWS selections are also stored locally:

~~~sh
ctx ls kube
ctx set kube client-a-dev --namespace payments
ctx ls aws
ctx set aws client-a
ctx ls gcloud
ctx set gcloud client-a

ctx run kubectl get pods
ctx run aws sts get-caller-identity
ctx run gcloud projects list
~~~

`ctx run kubectl` supplies `--context` and `--namespace`. Explicit kubectl flags
take priority. `ctx run aws` supplies `--profile`; AWS_PROFILE,
AWS_DEFAULT_PROFILE, or an explicit `--profile` takes priority. Use
`--kubeconfig /absolute/path` with `ctx set kube` when the context is in a
non-default kubeconfig. `ctx run gcloud` similarly applies `--configuration`;
CLOUDSDK_ACTIVE_CONFIG_NAME or an explicit `--configuration` takes priority.

Select a browser profile and open project URLs without mixing client, admin, and
personal sessions:

~~~sh
ctx ls browser
ctx set browser firefox:client-a
ctx open http://localhost:3000

ctx set browser 'chrome:Profile 1'
ctx open https://client-a.example
~~~

Firefox values are Firefox profile names. Chrome and Chromium values are profile
directory names such as `Default` or `Profile 1`; Safari currently exposes
`safari:default`. `ctx ls browser` asks every installed browser provider for its
available selections and prints values ready for `ctx set browser`. On macOS the
providers launch a new app instance through `open`; on other systems they invoke
the browser executable. `CTX_BROWSER` temporarily overrides the project selection.

Database adapters use native client-side profiles and do not copy connection
secrets into `.ctx`:

~~~sh
ctx set postgres client-a-dev
ctx run psql
ctx run pg_dump app > app.sql

ctx set mysql client-a
ctx run mysql app
ctx run mysqldump app > app.sql
~~~

PostgreSQL selections become `PGSERVICE` and can include `--service-file`.
Existing PGSERVICE and PGSERVICEFILE values take priority. MySQL selections use
login paths created separately with `mysql_config_editor`.

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

## Profile bundles

Put reusable bundles in `$HOME/.config/ctx/config.toml`:

~~~toml
[profiles."client-a"]
docker = "orbstack"
podman = "podman-machine-default"
nerdctl = "k8s.io"
kube_context = "client-a-dev"
kube_namespace = "payments"
aws_profile = "client-a"
gcloud_configuration = "client-a"
browser = "firefox:client-a"
postgres_service = "client-a-dev"
mysql_login_path = "client-a"
shell_path = "/opt/client-a/bin"

[profiles."client-a".env]
APP_ENV = "development"
AWS_REGION = "eu-west-1"
~~~

You can also create or update bundle entries from the CLI:

~~~sh
ctx profile set client-a docker orbstack
ctx profile set client-a kube_context client-a-dev
ctx profile set client-a kube_namespace payments
ctx profile set client-a aws_profile client-a
ctx profile set client-a gcloud_configuration client-a
ctx profile set client-a browser firefox:client-a
ctx profile set client-a postgres_service client-a-dev
ctx profile set client-a mysql_login_path client-a
ctx profile set client-a shell_path /opt/client-a/bin
ctx profile env client-a APP_ENV development
ctx profile show client-a
~~~

Select a bundle for the current project:

~~~sh
ctx profile ls
ctx profile use client-a
ctx explain
ctx doctor
~~~

The project `.ctx` contains only `profile = "client-a"`. Direct values in the
same `.ctx` override bundle values, so `ctx set docker desktop-linux` can make a
local exception. `ctx explain` shows the resolved value and its source. `ctx
doctor` checks that selected profiles and contexts still exist without making
network calls.

The same bundle can provide a declarative child-shell environment:

~~~sh
ctx env
ctx run -- npm test
ctx shell
ctx shell -- npm test
~~~

`ctx run --` applies the profile to one command. `ctx shell` starts the user's
shell with those values and restores the original environment when it exits.
Existing environment variables take priority over profile values. `shell_path`
is prepended to PATH. ctx does not source startup fragments or execute profile
hooks.

Profiles contain selectors, not credentials. Cloud keys, Kubernetes credentials,
database passwords, browser data, and tokens remain in each tool's native config
or credential store. Environment entries are plain text, so they are intended for
non-secret settings only.

## First-party and external adapters

The bundled first-party adapters live under `adapters/` in this repository:

- `docker`, `podman`, and `nerdctl` contain the built-in container-engine shims.
  The installer places those executables in the binary directory so normal engine
  commands can transparently resolve the current project's selection.
- `firefox`, `chrome`, `chromium`, and `safari` are browser providers used by the
  built-in `browser` context. Each provider owns application-specific profile
  discovery, validation, and launch behavior.
- `kube` owns `kubectl` contexts, namespaces, and kubeconfig selection.
- `aws` owns AWS CLI profiles.
- `gcloud` owns Google Cloud configurations.
- `postgres` owns PostgreSQL service profiles and client commands.
- `mysql` owns MySQL login paths and client commands.

The installer installs and trusts these packages alongside ctx. They use the same
public adapter protocol as third-party additions, so integrations can evolve
without adding another selector switch to the core.

ctx adapter API v1 lets a separately installed executable provide `list`,
`configure`, `validate`, `run`, `doctor`, and optional `open` operations.
Adapters are loaded only from `$CTX_HOME/adapters`; ctx never sources them or
discovers code in the current directory or arbitrary PATH entries.

Test, install, review, and trust an adapter explicitly:

~~~sh
ctx adapter test ./ctx-azure
ctx adapter install ./ctx-azure
ctx adapter inspect azure
ctx adapter trust azure
ctx adapter ls
~~~

`ctx adapter test` executes the adapter's `doctor` operation, so review unknown
adapter code before testing it. `ctx adapter install` only copies a structurally
valid package and does not execute it.

Installation alone does not permit execution. Trust records a checksum of every
file in the installed adapter. Editing any file revokes trust until the user
reviews it and runs `ctx adapter trust` again.

Once trusted, an external adapter behaves like a built-in selector:

~~~sh
ctx ls azure
ctx set azure client-a-subscription
ctx run azure account show
ctx adapter doctor azure
ctx explain
~~~

External selections also work in bundles:

~~~sh
ctx profile set client-a azure client-a-subscription
~~~

Adapters with the `open` capability can be invoked with `ctx open --adapter
NAME`. Use `ctx adapter remove NAME` to remove an installed adapter and its trust
record. The repository includes an executable reference adapter under
`examples/adapters/echo`. See the [adapter API v1 specification](docs/adapter-api.md)
to implement an adapter in any language.

An adapter with `kind = "browser"` extends the built-in browser context instead
of creating another top-level selector. Its name becomes the prefix in values
such as `brave:Default`; `ctx ls browser`, `ctx set browser`, `ctx open`, and
`ctx doctor` route through it automatically.

## Remove

Remove the installed ctx, docker, podman, and nerdctl wrappers from $HOME/.local/bin after checking they belong to ctx. Your config and installed adapters are in $HOME/.config/ctx. If you added the PATH line solely for ctx, remove that line from your shell startup file.

## License

MIT

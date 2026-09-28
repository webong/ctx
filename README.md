# ctx

Project-local contexts for the tools you already use.

`ctx` lets each project choose its own Docker context, Podman connection,
Kubernetes context, cloud profile, browser profile, database profile, and shell
environment—without changing global defaults.

```sh
cd my-project

ctx set docker orbstack
ctx set kube development --namespace payments
ctx set aws client-a
ctx set browser firefox:client-a

docker ps
ctx run kubectl get pods
ctx run aws sts get-caller-identity
ctx open http://localhost:3000
```

Selections are stored in a local `.ctx` file and automatically applied when a
command runs through ctx or one of its container shims.

## Why ctx?

- Keep development, client, and personal environments separate.
- Switch tool contexts per project instead of globally.
- Bundle several selections into a reusable profile.
- Use Docker, Podman, nerdctl, and Apple Container through one interface.
- Open URLs in the browser profile selected for the project.
- Extend ctx with trusted adapters instead of adding tool-specific logic to the
  core.
- Keep credentials in the native tools; ctx stores selectors, not secrets.

## Install

### macOS and Linux

Install the latest native release:

```sh
curl -fsSL https://raw.githubusercontent.com/webong/ctx/main/install.sh | sh
```

Ensure the install directory comes before the real container CLIs on `PATH`:

```sh
export PATH="$HOME/.local/bin:$PATH"
```

The installer verifies the release checksum and opens the adapter selection flow
when attached to a terminal. From a source checkout, it builds ctx locally:

```sh
./install.sh
```

For unattended installation:

```sh
./install.sh --adapters docker,kube,firefox
./install.sh --all
./install.sh --minimal
```

On Windows:

```powershell
.\install.ps1 -Interactive
.\install.ps1 -Adapters docker,kube,firefox
.\install.ps1 -AllAdapters
```

See [Cross-platform support](docs/cross-platform.md) for platform details. Set
`CTX_BIN_DIR` and `CTX_HOME` to use custom installation locations.

## Quick start

List the contexts available for a tool, select one in the current project, and
inspect the result:

```sh
ctx ls container
ctx ls docker
ctx set docker orbstack
ctx status
ctx explain
```

With the Docker, Podman, and nerdctl shims installed, normal commands use the
project selection:

```sh
docker ps
podman ps
nerdctl ps
```

Use `ctx run` for tools that do not have transparent shims:

```sh
ctx set kube development --namespace payments
ctx run kubectl get pods

ctx set aws client-a
ctx run aws sts get-caller-identity

ctx set gcloud client-a
ctx run gcloud projects list
```

Explicit CLI flags and environment variables still take priority over ctx.

## Browser and database contexts

Choose a browser profile and open project URLs in it:

```sh
ctx ls browser
ctx set browser 'chrome:Profile 1'
ctx open http://localhost:3000
```

Database adapters use profiles maintained by the database clients. Passwords are
not copied into `.ctx`.

```sh
ctx set postgres client-a-dev
ctx run psql app

ctx set mysql client-a
ctx run mysql app
```

## Profiles and shell environments

A profile groups several context selections and non-secret environment values:

```sh
ctx profile set client-a docker orbstack
ctx profile set client-a kube_context client-a-dev
ctx profile set client-a kube_namespace payments
ctx profile set client-a aws_profile client-a
ctx profile set client-a browser firefox:client-a
ctx profile env client-a APP_ENV development

ctx profile use client-a
ctx profile show client-a
ctx doctor
```

Apply the profile environment to one command or start a child shell:

```sh
ctx run -- npm test
ctx shell
ctx shell -- npm test
```

Environment values are stored as plain text. Use them for ordinary configuration,
not passwords, tokens, or private keys.

## Images, volumes, and build caches

Share a registry-backed build cache while keeping each engine's cache format
separate:

```sh
ctx build docker \
  --cache-ref ghcr.io/acme/api:docker-cache \
  -- --tag ghcr.io/acme/api:dev .
```

Copy images between supported engines through a temporary archive:

```sh
ctx image copy \
  docker:orbstack \
  podman:podman-machine-default \
  acme/api:dev
```

Copy a named volume between engines:

```sh
ctx volume copy \
  docker:orbstack \
  podman:podman-machine-default \
  postgres-data \
  postgres-data
```

Volume copy is a point-in-time migration, not live synchronization. Stop or
quiesce databases first. ctx refuses to import into an existing target volume.

Apple Container image imports require a version newer than 1.3.0 because of
[GHSA-r3h2-rgqf-9hv9](https://github.com/apple/containerization/security/advisories/GHSA-r3h2-rgqf-9hv9).

## Adapters

ctx ships maintained adapters for:

| Family | Adapters |
| --- | --- |
| Containers | Docker, Podman, nerdctl/containerd, Apple Container |
| Browsers | Firefox, Chrome, Chromium, Safari |
| Cloud and orchestration | Kubernetes, AWS, gcloud |
| Databases | PostgreSQL, MySQL |

The native installer places bundled adapters in a local catalog. Install only
what you need:

```sh
ctx setup
ctx adapter available
ctx adapter add podman postgres
ctx adapter remove firefox
ctx adapter refresh
```

External adapters use the same API and are untrusted until explicitly reviewed
and trusted:

```sh
ctx adapter test ./ctx-azure
ctx adapter install ./ctx-azure
ctx adapter inspect azure
ctx adapter trust azure
```

See [Adapters](adapters/README.md) for the bundled packages and
[Adapter API v1](docs/adapter-api.md) to build an integration.

## Command reference

| Command | Purpose |
| --- | --- |
| `ctx ls <selector>` | List available contexts |
| `ctx set <selector> <name>` | Select a context for the current project |
| `ctx clear [selector]` | Remove one or all project selections |
| `ctx status` | Show active selections |
| `ctx explain` | Show resolved values and their sources |
| `ctx run <tool> ...` | Run a tool with its selected context |
| `ctx run -- <command> ...` | Run any command with the profile environment |
| `ctx shell` | Start a child shell with the profile environment |
| `ctx open <url>` | Open a URL with the selected browser profile |
| `ctx doctor` | Validate configured selections and adapters |
| `ctx hook <bash|zsh|powershell>` | Generate an optional shell prompt observer |
| `ctx graph status` | Show the local system graph revision and size |
| `ctx graph vertices [kind]` | Inspect observed graph vertices |
| `ctx graph edges [relationship]` | Inspect observed graph relationships |
| `ctx graph snapshot` | Export the CTX system graph as JSON |
| `ctx graph changes [cursor]` | Read graph changes after a cursor |

Run `ctx` without arguments for the complete command list.

## System graph

CTX stores observed system context in `$CTX_HOME/graph.json` (by default,
`$HOME/.config/ctx/graph.json`). A CTX command records the invoking shell's
parent process, current directory, detected project, and active profile.
Successful `ctx open` calls record the browser provider, profile, and URL
origin; paths, query strings, and fragments are omitted. Environment values,
shell history, and browser credentials are not collected.

The `ctx graph` commands are short-lived readers of this durable graph. The
public `github.com/webong/ctx/graph` package supports other services registering
their own namespaces and validators. The CTX namespace records CTX observations;
it does not grant permissions or trust to other services.

To update shell location context after each prompt, opt in to a prompt hook:

```sh
eval "$(ctx hook bash)" # or: eval "$(ctx hook zsh)"
```

In PowerShell, add `ctx hook powershell | Out-String | Invoke-Expression` to
your PowerShell profile.

The hook starts a short-lived observation command. Browser activity is currently
observed when URLs are opened through `ctx open`; direct navigation in other
browser windows is not collected yet.

## Configuration

Project selections live in `.ctx`. In Git repositories, ctx adds that file to
the repository's local exclude list instead of modifying `.gitignore`.

Global defaults, project mappings, profiles, and profile environments live in:

```text
$HOME/.config/ctx/config.toml
```

Resolution follows this order:

1. Explicit CLI flags or tool-specific environment variables.
2. The nearest `.ctx` file or central project mapping.
3. Automatic Docker runtime detection on macOS.
4. A configured global fallback.
5. The underlying tool's own default.

## Migrating from dctx

Remove any old `eval "$(dctx hook zsh)"` or `eval "$(dctx hook bash)"` line
from your shell startup file. The old hook may export `DOCKER_CONTEXT` and
override ctx. Recreate project selections with `ctx set`; dctx configuration is
not imported automatically.

## Uninstall

Remove the ctx-owned executables from `$HOME/.local/bin`. Configuration and
installed adapters are stored under `$HOME/.config/ctx` unless `CTX_HOME` was
changed.

## License

[MIT](LICENSE)

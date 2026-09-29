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
ctx set firefox:client-a

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

Adapters describe one runtime—`computer`, `virtualizer`, or `browser`—and one
or more interaction surfaces: `shell` and `web`. Their capabilities decide what
ctx can list, run, open, or share. This keeps tool-specific behavior in adapter
packages while the core provides common discovery and routing.

## What works today

| Area | Current capability |
| --- | --- |
| Project contexts | Select native tool contexts in `.ctx`, group selections in profiles, inspect resolution, and apply profile environment values to a command or child shell. |
| Computer tools | Route Kubernetes, AWS, gcloud, PostgreSQL, and MySQL commands through their selected contexts. Claude Code and Codex adapters provide CLI shims, project hooks, and native plugin delegation. |
| Virtualizers | Route Docker, Podman, nerdctl, and Apple Container commands. Register named engine connections, use supported registry build caches, and transfer images or named volumes between engines. |
| Browsers | Open URLs in a selected Firefox, Chrome, Chromium, or Safari profile. Browser adapters can share the resources listed [below](#browser-sharing). |
| System graph | Scan trusted adapters for available contexts and capabilities, resolve usable providers, and inspect or export the local inventory. Other services can import the graph and supervisor Go packages. |
| Extensions | Install bundled or third-party adapters. A prebuilt, platform-specific adapter archive works with a bare ctx binary; building an adapter from Go source requires Go. |

Available operations depend on the installed, trusted adapter and its native
tool. Use `ctx adapter inspect <name>` and `ctx share:browser capabilities
--from <browser:profile>` to inspect a particular installation.

## Install

### macOS and Linux

Install from a source checkout with Go 1.23 or newer:

```sh
git clone https://github.com/webong/ctx.git
cd ctx
./install.sh
export PATH="$HOME/.local/bin:$PATH"
```

The installer builds ctx and its Go browser helpers, then opens adapter
selection when attached to a terminal. Put the install directory before the
real container CLIs on `PATH`. For unattended installation, run one of these
from the checkout:

```sh
./install.sh --adapters docker,kube,firefox
./install.sh --all
./install.sh --minimal
```

On Windows, install from a source checkout with Go 1.23 or newer:

```powershell
git clone https://github.com/webong/ctx.git
Set-Location ctx
.\install.ps1 -Interactive
$env:PATH = (Join-Path $env:LOCALAPPDATA 'Programs\ctx\bin') + ';' + $env:PATH
```

For unattended Windows setup, use `-Adapters docker,kube,firefox` or
`-AllAdapters` in place of `-Interactive`.
The Windows installer writes PowerShell completion to
`$env:APPDATA\ctx\ctx-completion.ps1` by default. Add this line to your
PowerShell profile to enable it:

```powershell
. "$env:APPDATA\ctx\ctx-completion.ps1"
```

When a prebuilt release is available, macOS and Linux can download it without
Go:

```sh
curl -fsSL https://raw.githubusercontent.com/webong/ctx/main/install.sh | sh
```

Windows can download the installer script to a temporary file and run it:

```powershell
$ctxInstaller = Join-Path $env:TEMP 'ctx-install.ps1'
Invoke-WebRequest https://raw.githubusercontent.com/webong/ctx/main/install.ps1 -OutFile $ctxInstaller
& $ctxInstaller -Interactive
```

The remote installers verify release checksums. If GitHub Releases has no
published version, use the source checkout instructions above.

See [Cross-platform support](docs/cross-platform.md) for platform details. Set
`CTX_BIN_DIR` and `CTX_HOME` to use custom installation locations.

## Quick start

List the contexts available for a tool, select one in the current project, and
inspect the result:

```sh
ctx ls virtualizer
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
`ctx real docker` shows which Docker executable the ctx shim will launch.

## Browser and database contexts

Choose a browser profile and open project URLs in it:

```sh
ctx ls browser
ctx set 'chrome:Profile 1'
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

Use `ctx profile ls` to list profiles, `ctx profile unset <name> <key>` or
`ctx profile env-unset <name> <variable>` to remove individual values, and
`ctx profile clear` to stop using the project profile. A profile can also set
`shell_path` to prepend a directory to `PATH` for commands launched through
ctx.

Apply the profile environment to one command or start a child shell:

```sh
ctx run -- npm test
ctx shell
ctx shell -- npm test
```

`ctx env` prints the environment additions for the current profile, and
`ctx shell --shell <executable>` chooses the child shell explicitly. `CTX_SHELL`
sets the default child shell when `--shell` is omitted.

Environment values are stored as plain text. Use them for ordinary configuration,
not passwords, tokens, or private keys.

## Computer-side AI CLIs

Computer integrations use the `computer` runtime on the `shell` surface and
declare CLI shims, hooks, and plugins in their manifest. The installer catalog
includes maintained Claude Code and Codex packages. Activate one or both with:

```sh
ctx setup --adapters claude_code,codex
ctx adapter ls computer
```

Put the ctx binary directory first on `PATH`, then use either CLI's normal
command:

```sh
claude
codex
```

The shims launch the real CLIs through ctx. Install hooks for the current
project with a local handler executable:

```sh
ctx computer hooks install claude_code --handler ./scripts/ctx-policy
ctx computer hooks install codex --events PreToolUse,PermissionRequest --handler ./scripts/ctx-policy
```

This writes native hook entries to `.claude/settings.local.json` or
`.codex/hooks.json`, and stores the handler and event list in the project's
git-ignored `.ctx` file. The event name is passed to the handler as its first
argument, and native hook JSON flows through stdin/stdout. Preview generated
settings with `ctx computer hooks print claude_code`; remove ctx-managed hooks
with `ctx computer hooks remove claude_code`. `ctx plugin computer <adapter>
...` delegates plugin and marketplace operations to the CLI's native plugin
command, where provider-specific project scope is supported. See [the adapter
API](docs/adapter-api.md#computer-side-cli-integrations) for details. The design
follows the runtime-neutral decision boundary and local operator controls described by
[Neura for Builders](https://www.neurarelay.com/builders) and
[Neura Local settings](https://www.neurarelay.com/operators#neura-local-settings).

## Virtualizer sharing and build caches

`ctx graph scan` discovers installed adapters and their declared capabilities.
Trusted browser and virtualizer adapters can also supply named contexts. The
inventory is available through `ctx graph vertices adapter`, `ctx graph
vertices capability`, and `ctx graph vertices context`; it does not presume
which providers or host products are installed.

You can give an engine connection a convenient alias. `--provider` names a
trusted ctx adapter, and `--selection` is that adapter's native context,
connection, or namespace. `--virtualizer` and `--machine` add descriptive
metadata when known; neither is required by the core.

```sh
ctx virtualizer add orb --virtualizer orbstack --provider docker --selection orbstack
ctx virtualizer add desktop --virtualizer docker-desktop --provider docker --selection desktop-linux
ctx virtualizer add dev-vm --virtualizer utm --machine dev-vm --provider docker --selection utm-dev
ctx virtualizer add local-apple --virtualizer apple-container --provider apple --selection local
ctx virtualizer ls
ctx virtualizer show dev-vm
```

For the maintained nerdctl adapter, pin the daemon address separately from its
namespace:

```sh
ctx virtualizer add local-containerd --virtualizer containerd --provider nerdctl \
  --selection default --address /run/containerd/containerd.sock
```

The `utm-dev` selection above must already be a Docker context pointing at the
engine inside that UTM VM. For Podman in a VM, register its Podman connection
instead. These registrations describe container engines inside VMs; they do
not copy UTM VM disks or snapshots. Registered endpoints use `@name` in share
commands. `ctx` shows the resolved source and target before transferring.
Native `provider:selection` endpoints continue to work without registration.
The graph records aliases as declarations and contexts as adapter observations;
share commands still validate the selected endpoint before transferring.

Share a registry-backed build cache using a registered instance:

```sh
ctx build @orb \
  --cache-ref ghcr.io/acme/api:docker-cache \
  -- --tag ghcr.io/acme/api:dev .
```

Copy images between supported engines through a temporary archive:

```sh
ctx share:virtualizer image copy \
  @orb \
  @desktop \
  acme/api:dev
```

Copy a named volume between engines:

```sh
ctx share:virtualizer volume copy \
  @orb \
  @dev-vm \
  postgres-data \
  postgres-data
```

`image sync` pushes and pulls a registry reference when both adapters support
it, and falls back to an archive transfer otherwise. `image copy` always uses
an archive. `ctx share:container` is an alias for `ctx share:virtualizer`.
You can also stream a named volume through a file or pipe:

```sh
ctx share:virtualizer volume export @orb postgres-data > postgres-data.tar
ctx share:virtualizer volume import @desktop restored-data < postgres-data.tar
```

Volume copy is a point-in-time migration, not live synchronization. Stop or
quiesce databases first. ctx refuses to import into an existing target volume.

Docker, Podman, and nerdctl adapters declare build, registry image transfer,
archive image transfer, and named-volume transfer capabilities. Apple Container
declares archive image and named-volume transfer, but no build or registry
push/pull capability. Apple Container image imports require a version newer
than 1.3.0 because of
[GHSA-r3h2-rgqf-9hv9](https://github.com/apple/containerization/security/advisories/GHSA-r3h2-rgqf-9hv9).

## Browser sharing

`ctx share:browser` bridges browser resources through installed adapters.
Its versioned JSON contract is documented in the adapter API. External Go
adapters can use `github.com/webong/ctx/adapter/browser` for the shared types
and validation helpers. CTX core does not contain browser storage or
platform-specific code.

| Bundled adapter | Declared share operations |
| --- | --- |
| Firefox | Cookie list, export, and import; policy export; certificate list, export, and import |
| Chrome and Chromium | Cookie list, export, and import; policy export |
| Safari | Policy export |

The adapter's declared operations are the starting point; OS encryption and
profile state can further limit an individual transfer.

Firefox, Chrome, and Chromium can list and export a selected site cookie and
import a supported cookie into a closed profile. Source and target may be
different browser providers. It runs as a shell command and does not require
a browser extension:

```sh
ctx share:browser cookie list --from firefox:personal --site https://example.com
ctx share:browser cookie copy --from firefox:personal --site https://example.com --name session --to-profile firefox:work
ctx share:browser cookie copy --from firefox:personal --site https://example.com --name session --to-file ./session-cookie.json
ctx share:browser cookie copy --from firefox:personal --site https://example.com --name session --stdout | consumer
ctx share:browser cookie list --from chrome:Default --site https://example.com
ctx share:browser cookie copy --from chrome:Default --site https://example.com --name session --to-file ./chrome-cookie.json
ctx share:browser cookie copy --from chromium:Default --site https://example.com --name session --stdout | consumer
ctx share:browser cookie copy --from firefox:personal --site https://example.com --name session --to-profile chromium:Default
ctx share:browser cookie import --from-file ./session-cookie.json --to-profile chrome:Profile\ 1
ctx share:browser policy export --from chrome:Default --to-file ./chrome-policies.json
ctx share:browser capabilities --from safari:default
ctx share:browser certificate list --from firefox:personal
ctx share:browser certificate copy --from firefox:personal --to-profile firefox:work -- --nickname 'Client Identity' --password-file ./identity.pass
```

`--from` defaults to the selected browser. Listing prints cookie metadata, not
values, and does not unlock the OS cookie key. The site URL selects the scheme
and host; use `--path` to select an exact cookie path. A site can have cookies
with the same name in different domains, paths, or partitions. Use `--id` from
`cookie list` to select an exact row, or narrow Firefox cookies with
`--attribute firefox.origin_attributes=<value>`. Listing is tab-separated and includes name, domain,
path, expiry, SameSite policy, row ID, and partition scope. Files are created
with mode 0600 and are plain JSON containing `version`, `source`, `site`, and a
`cookie` object with its value and scope fields. `same_site_policy` is the
portable value; browser-specific fields are namespaced under `attributes`.
`--stdout` requires a pipe; use `--to-file` for a protected file.

Chrome and Chromium export reads the profile's committed SQLite cookies. On
macOS, encrypted cookies require access to the browser's Safe Storage item in
Keychain; the source adapter requests it only after a cookie is selected and the output is
valid. On Linux, v10 cookies can be decoded locally; v11 cookies use Secret
Service through `secret-tool` or KWallet through `kwallet-query` (set
`CTX_KWALLET_NAME` for a non-default wallet). On Windows, legacy DPAPI and
AES-GCM cookies can be exported for the current user. Chrome App-Bound (`v20`)
cookies cannot be exported by a standalone ctx process. Unsupported encryption
versions and unavailable OS keys fail without writing a partial bundle.

Chrome and Chromium profile import encrypts the selected cookie for the target
profile on macOS or Linux. The browser must be closed; `lsof` is required, and
the adapter rejects an existing Chromium `SingletonLock`. On Linux, it uses the
target's existing v10 or v11 format, or v11 when a wallet key is available.
Windows profile import is unavailable for browser-bound encryption. Cross-browser
copy supports unpartitioned cookies when the target can represent their scope;
otherwise the target adapter rejects the import. `cookie import` accepts a
previously exported file or `--stdin` from a pipe. Safari's normal browsing
cookies remain inaccessible through a supported shell interface.

Firefox profile import requires `sqlite3` and `lsof`, a closed target profile,
and an unpartitioned cookie. It refuses to overwrite an existing target cookie
unless `--replace` is given. The read-only
list and file/pipe forms need `sqlite3`; they can read committed cookies while
the browser is open, though recent in-memory changes may not yet appear. When a
read-only SQLite connection cannot open a write-ahead log, the adapter makes a private
temporary database snapshot and removes it on normal completion. A forced
process termination can leave that snapshot in the system temporary directory.
`policy export` collects available machine policy files or registry entries
from the selected browser adapter into a mode-0600 bundle. Policies are usually
machine or user managed rather than profile data; export does not apply them to
another browser. The adapter protocol permits additional resource operations.
Browser encryption keys and non-exportable private keys are not transferable.
Firefox can share an exportable client certificate and its private key through
a password-protected PKCS#12 bundle when `certutil` and `pk12util` are installed.
Use `-- --nickname <name> --password-file <mode-0600-file>` for export or copy;
import needs `-- --password-file <mode-0600-file>`. The source and target Firefox
profiles must be closed. A hardware-backed or otherwise non-exportable key
fails in the NSS tool without a partial bundle.

Third-party browser adapters can register `resource.list`, `resource.export`,
and `resource.import` under `browser_share`. `ctx share:browser <resource>` then
routes list, file/pipe export, profile copy, and file/pipe import through a
versioned JSON bundle without a core change. The destination adapter decides
whether it can represent the source resource.

`ctx share:computer` is reserved for sharing a computer context. Adapters can
register other spaces through a `share` capability and optional `share_spaces`
manifest field, exposed as `ctx share:<space> ...`.

## Adapters

ctx ships maintained adapters for:

| Family | Adapters |
| --- | --- |
| Virtualizers | Docker, Podman, nerdctl/containerd, Apple Container |
| Browsers | Firefox, Chrome, Chromium, Safari |
| Cloud and orchestration | Kubernetes, AWS, gcloud |
| Databases | PostgreSQL, MySQL |
| Computer integrations | Shell AI CLI shims, hooks, and plugins |

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

A bare `ctx` binary can also install a prebuilt adapter archive without Go.
The archive includes the manifest and an executable for the current platform:

```sh
ctx adapter install ./azure-darwin-arm64.ctxadapter
ctx adapter trust azure
```

For a published multi-platform adapter index, pin the index checksum. ctx picks
the current platform and verifies the selected archive's checksum from the index:

```sh
ctx adapter install https://example.com/azure.ctxadapter.json --sha256 "$INDEX_SHA256"
ctx adapter trust azure
```

Go adapter authors build the same archive explicitly with `ctx adapter build
./my-adapter`; this command requires Go. See the [Go example](examples/adapters/go_echo)
and [adapter packaging guide](docs/adapter-api.md#binary-packages-and-go-builds).

See [Adapters](adapters/README.md) for the bundled packages and
[Adapter API v2.0](docs/adapter-api.md) to build an integration.

## Command reference

| Command | Purpose |
| --- | --- |
| `ctx version` | Show the installed ctx version |
| `ctx ls <runtime-or-selector>` | List available contexts |
| `ctx set <adapter>:<selection>` | Select a context using its adapter's declared runtime |
| `ctx set <selector> <name>` | Select a context with the existing two-argument form |
| `ctx clear [selector]` | Remove one or all project selections |
| `ctx profile <ls|show|use|set|unset|env|env-unset|clear> ...` | Manage named profiles and their environment values |
| `ctx status` | Show active selections |
| `ctx resolve <key>` | Print one resolved configuration value |
| `ctx explain` | Show resolved values and their sources |
| `ctx env` | Print environment additions for the active profile |
| `ctx real <command>` | Find the native executable behind a ctx shim |
| `ctx run <tool> ...` | Run a tool with its selected context |
| `ctx run -- <command> ...` | Run any command with the profile environment |
| `ctx shell [--shell <executable>]` | Start a child shell with the profile environment |
| `ctx open <url>` | Open a URL with the selected browser profile |
| `ctx hook computer <adapter> <event>` | Pass a computer hook event through a trusted adapter |
| `ctx computer hooks <print|install|remove> ...` | Manage ctx entries in supported AI CLI project settings |
| `ctx plugin computer <adapter> ...` | Run a computer integration's plugin operation |
| `ctx setup [--all|--minimal|--adapters <names>]` | Select bundled adapters to activate |
| `ctx adapter <ls|available|add|refresh|inspect|trust|test|doctor|remove> ...` | Inspect and manage adapter packages |
| `ctx adapter build <source>` | Compile a Go adapter into a platform archive |
| `ctx adapter pack <directory>` | Archive an already built adapter package |
| `ctx adapter index <output> <archives...>` | Create a platform index with archive checksums |
| `ctx adapter install <source>` | Install a local directory, archive, or pinned HTTPS package |
| `ctx virtualizer <add|ls|show|remove> ...` | Register named virtualizer instances for sharing and builds |
| `ctx build [provider|@instance] --cache-ref <ref> -- <args>` | Build with a registry-backed cache on a capable virtualizer |
| `ctx share:virtualizer image <sync|copy> ...` | Transfer images through installed virtualizer providers |
| `ctx share:virtualizer volume <export|import|copy> ...` | Transfer named volumes through installed virtualizer providers |
| `ctx share:browser cookie <list|copy|import> ...` | List, export, or import one site cookie through browser adapters |
| `ctx share:browser policy export ...` | Export browser policy sources to a protected bundle or pipe |
| `ctx share:browser certificate <list|export|copy|import> ...` | Share an exportable certificate through a capable browser adapter |
| `ctx share:browser capabilities ...` | Show a browser adapter's share operations |
| `ctx share:<space> ...` | Invoke a trusted adapter's registered share operation |
| `ctx doctor` | Validate configured selections and adapters |
| `ctx hook <bash|zsh|powershell>` | Generate an optional shell prompt observer |
| `ctx completion powershell` | Generate dynamic PowerShell command completion |
| `ctx graph status` | Show the local system graph revision and size |
| `ctx graph scan` | Refresh the machine's adapter, capability, and context inventory |
| `ctx graph resolve [runtime|all] [capability...]` | Discover usable contexts from the refreshed graph |
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

`ctx graph scan` asks trusted installed adapters for their available contexts
and ordinary resource metadata. Adapters declaring `observe` return versioned
JSON; older browser and virtualizer adapters use their line-oriented `list`
output. The scan reconciles removed contexts and records when it ran.
`ctx graph resolve virtualizer image_save` returns contexts that currently offer that
capability. `ctx ls`, `ctx status`, browser share, virtualizer share, and build
read the graph projection; adapter trust and live validation still govern each
operation.
`ctx status` marks a selected browser or adapter context `[observed]` when the
current graph scan found it.

An unqualified virtualizer context resolves to the sole observed provider when
there is one; ambiguous names require `provider:context` or a registered
`@instance`. Build can choose the sole observed provider that offers `build`.

The public `github.com/webong/ctx/graph` package supports other services
registering their own namespaces and validators. The public
`github.com/webong/ctx/graph/system` package supplies CTX's system inventory
projection. Observations inform discovery; adapters still validate and route
each operation.

Services can also import `github.com/webong/ctx/supervisor` for approved local
processes. It verifies declared executable checksums, isolates process trees,
tracks leases and orphan recovery, gates readiness on caller-supplied endpoint
and protocol checks, and projects bounded lifecycle history into the graph.
Callers retain their own authorization, process admission, and protocol rules.
See [the graph and supervisor contract](docs/adr-graph-runtime.md) for the API
and recovery behavior.

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

Global defaults, project mappings, profiles, and profile environments live in
`$HOME/.config/ctx/config.toml` on macOS and Linux, and
`$env:APPDATA\ctx\config.toml` on Windows. `CTX_HOME` overrides the containing
directory. A virtualizer's global fallback can be set with
`ctx set docker <context> --global` (or another capable virtualizer adapter).

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

Remove the ctx-owned executables from `$HOME/.local/bin` on macOS and Linux, or
`$env:LOCALAPPDATA\Programs\ctx\bin` on Windows. Configuration and installed
adapters are stored under `$HOME/.config/ctx` on macOS and Linux, or
`$env:APPDATA\ctx` on Windows, unless `CTX_HOME` was changed.

## License

[MIT](LICENSE)

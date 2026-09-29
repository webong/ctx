# ctx adapter API v1

An external adapter is a directory containing `adapter.toml` and at least one
executable.
ctx installs adapters under `$CTX_HOME/adapters`; it never discovers or sources
code from the current directory or arbitrary `PATH` entries.

## Manifest

~~~toml
api_version = "1"
name = "example"
kind = "selector"
executable = "ctx-example"
# Optional native Windows implementation. When absent, executable is used.
executable_windows = "ctx-example.ps1"
description = "Example context adapter"
capabilities = "list,validate,run,doctor,open,share"
selector_key = "example_context"
extra_keys = "example_namespace"
commands = "example,examplectl"
# Optional when capabilities includes share; defaults to adapter name.
share_spaces = "workspace,project"
# Native variables that take priority over the stored selection.
override_env = "EXAMPLE_CONTEXT,EXAMPLE_HOST"
# Optional for kind = "container". At most one installed provider should set it.
default_provider = "false"
~~~

Names use lowercase letters, numbers, and underscores, and cannot collide with a
context family or ctx command. `validate` and `doctor` are required. Selector,
browser, and container adapters also require `run` or `open`; computer shims
require `run`. `list` is optional.

`kind` defaults to `selector` when no computer endpoint is declared. A selector
adapter adds a top-level ctx selector.
A `browser` adapter instead provides an engine to the built-in browser context;
its `list` output uses `name:profile` values and `validate`, `doctor`, and `open`
receive the profile portion as their selection.

A `container` adapter provides an engine to the built-in container family. Its
unqualified `list` output contains native context, connection, or namespace
names; `ctx ls container` prefixes each result as `name:selection`. The provider
may implement `build`, image, and volume capabilities in addition to normal
`run` routing. `ctx share:container image copy` and
`ctx share:container volume copy` resolve qualified endpoints
dynamically, so third-party container providers need no core changes.

`selector_key` defaults to the adapter name for selector and container
adapters, and to `browser` for browser adapters. `extra_keys` declares
additional profile values owned by the adapter. `commands` defaults to the
adapter name for selector and container adapters and lets `ctx run` route
native command names to the adapter. `override_env` declares native environment
variables, in precedence order, that `ctx status` should report instead of the
stored selection. The provider remains responsible for honoring them during
`run`. `default_provider`
lets backward-compatible unqualified build, image-sync, and
volume endpoints choose a provider without hard-coding an engine in the core;
multiple trusted defaults are reported as an error.

## Computer-side CLI integrations

Computer integrations declare their host commands and capabilities directly,
without a dedicated manifest kind. For example, a Claude package can declare:

~~~toml
name = "claude_code"
computer_commands = "claude"
computer_capabilities = "hook,plugin"
capabilities = "validate,doctor,run"
~~~

The package includes a `claude` launcher file that delegates to
`ctx run claude "$@"`. When trusted, ctx installs that launcher beside `ctx`.
It locates the real CLI with the shim directory excluded from `PATH` and passes
its absolute path in `CTX_ADAPTER_REAL_COMMAND`, preventing recursive shim
calls. Computer commands do not require a project selection; the adapter still
receives the active project and profile environment.
For a Codex integration, declare `computer_commands = "codex"` and include the
matching `codex` launcher in its package.

`computer_capabilities` currently accepts `hook` and `plugin`.
`ctx hook computer <adapter> <event>` passes the native hook payload from stdin
to the adapter as `hook -- EVENT`; stdout, stderr, and exit status flow back to
the CLI runtime. The adapter owns the event schema and response contract.
`ctx plugin computer <adapter> [ARGUMENTS...]` invokes
`plugin -- ARGUMENTS...` for provider-specific extension setup. These operations
use the same adapter trust and checksum checks as shims. The core does not parse
or rewrite provider hook or plugin configuration files.

The bundled `claude_code` and `codex` packages install `claude` and `codex`
shims. Put ctx's bin directory before the vendor CLI directory on `PATH`, then
activate them with `ctx setup --adapters claude_code,codex`. `ctx run claude
--version` and `ctx run codex --version` reach the installed CLIs. For hook
experiments, set `CTX_COMPUTER_HOOK_COMMAND` to a handler executable; ctx passes
the event name as its first argument and preserves the vendor's JSON stdin,
stdout, stderr, and exit status. Plugin operations delegate to each CLI's own
plugin command:

~~~sh
ctx plugin computer claude_code marketplace list
ctx plugin computer codex marketplace list
~~~

For a local hook smoke test, add a native command hook to the CLI's own settings.
For example, this Claude Code entry can go in `.claude/settings.local.json`:

~~~json
{
  "hooks": {
    "PreToolUse": [{
      "matcher": "*",
      "hooks": [{
        "type": "command",
        "command": "ctx hook computer claude_code PreToolUse"
      }]
    }]
  }
}
~~~

Codex reads the equivalent event from `~/.codex/hooks.json`:

~~~json
{
  "hooks": {
    "PreToolUse": [{
      "matcher": "*",
      "hooks": [{
        "type": "command",
        "command": "ctx hook computer codex PreToolUse"
      }]
    }]
  }
}
~~~

Both hook declarations require `CTX_COMPUTER_HOOK_COMMAND` to point to a local
handler before starting the CLI. The handler receives the event name as argv[1]
and the native hook JSON on stdin. These examples are user-side CLI settings;
the adapter package itself does not write to either CLI's configuration.

This boundary matches Neura Relay's builder flow: an action is reviewed, a
decision receipt is returned, and the CLI runtime decides what to do next.
Local settings can independently control agent authority, outside actions, and
sensitive changes. See [Neura for Builders](https://www.neurarelay.com/builders)
and [Neura Local settings](https://www.neurarelay.com/operators#neura-local-settings).

## Process protocol

ctx selects `executable_windows` on Windows when it is present and otherwise uses
`executable`. Windows adapters may be `.exe`, `.cmd`, `.bat`, or `.ps1`; PowerShell
scripts are launched without loading the user's profile.

ctx invokes the selected executable with one of these forms:

~~~text
ctx-example list
ctx-example validate SELECTION
ctx-example configure SELECTION -- OPTIONS...
ctx-example doctor SELECTION
ctx-example run SELECTION -- ARGUMENTS...
ctx-example open SELECTION -- ARGUMENTS...
ctx-example share SELECTION -- SPACE ARGUMENTS...
ctx-example hook SELECTION -- EVENT
ctx-example plugin SELECTION -- ARGUMENTS...
~~~

Computer endpoint operations omit the selection argument when none applies, so
their forms are `run -- ARGUMENTS...`, `hook -- EVENT`, and
`plugin -- ARGUMENTS...`.

Every declared capability is invoked by the same protocol:

~~~text
ctx-example CAPABILITY SELECTION -- ARGUMENTS...
~~~

Container capabilities currently understood by the core are `build`,
`image_push`, `image_pull`, `image_save`, `image_load`, `volume_exists`,
`volume_create`, `volume_export`, and `volume_import`. Image save arguments are
`ARCHIVE IMAGE...`; image load receives `ARCHIVE`. Volume export writes a tar
stream to stdout, while volume import reads a tar stream from stdin.

Container resource transfers are exposed through `ctx share:container`.
The installed container adapters register the actual image and volume
capabilities, and ctx rejects a transfer when either endpoint lacks a needed
capability. `ctx share:browser cookie` is a shell-only Firefox reader and local
writer implemented by ctx. It uses a trusted installed Firefox browser adapter
for provider identity, but cookie access uses the native profile database and
does not invoke a browser extension. It can write one cookie to another closed
Firefox profile, a new mode-0600 JSON file, or stdout for a pipe. Other browser
engines and browser policies and keys still need provider-specific support.
`ctx share:computer` is also reserved. Any other installed adapter can declare
the `share` capability and optional `share_spaces` to register
`ctx share:<space>` commands. When `share_spaces` is omitted, the adapter name
is the space. CTX rejects ambiguous registrations, then invokes the trusted
adapter's `share` operation with the resolved selection. The first argument
after `--` is the space, followed by the user's remaining arguments; the
adapter owns their meaning.

The process receives `CTX_ADAPTER_API`, `CTX_ADAPTER_NAME`,
`CTX_ADAPTER_COMMAND`, `CTX_ADAPTER_REAL_COMMAND`, `CTX_PROJECT_DIR`, and
`CTX_PROFILE`. For container providers, `CTX_ADAPTER_REAL_COMMAND` is the resolved
underlying CLI path with ctx's transparent shim excluded. Values for declared
keys are exported as `CTX_ADAPTER_VALUE_<UPPERCASE_KEY>`. A `configure` operation
prints tab-separated `key<TAB>value` records for declared keys; ctx validates and
writes them to `.ctx`. Selections are identifiers, never credentials. Exit status 0
means success, 1 means validation or runtime failure, 2 means adapter usage error,
and 127 should identify a missing dependency. Normal data belongs on stdout;
diagnostics belong on stderr.

Adapters are separate processes and must not expect their environment changes to
affect ctx or its parent shell. They should use the selected native profile or
context when launching their underlying tool.

## Lifecycle

~~~sh
ctx adapter available
ctx adapter add example
ctx adapter refresh
ctx adapter test ./example
ctx adapter install ./example
ctx adapter inspect example
ctx adapter trust example
ctx adapter doctor example
ctx adapter remove example
~~~

The installer-provided catalog lives under `$CTX_HOME/catalog/adapters` (or
`CTX_CATALOG_HOME`). `ctx setup` and `ctx adapter add` copy explicitly selected
catalog packages into the active adapter store and trust their checksums. Catalog
membership is installer metadata, not a privilege or manifest property.

Direct `ctx adapter install ./directory` installation does not imply trust. Trust
records a checksum over every file in the adapter directory. Any subsequent
change makes the adapter untrusted until the user reviews and trusts it again.

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
capabilities = "list,validate,run,doctor,open"
selector_key = "example_context"
extra_keys = "example_namespace"
commands = "example,examplectl"
# Native variables that take priority over the stored selection.
override_env = "EXAMPLE_CONTEXT,EXAMPLE_HOST"
# Optional for kind = "container". At most one installed provider should set it.
default_provider = "false"
~~~

Names use lowercase letters, numbers, and underscores, and cannot collide with a
context family or ctx command. `validate` and `doctor` are required. At least
one of `run` and `open` is required. `list` is optional.

`kind` defaults to `selector`. A selector adapter adds a top-level ctx selector.
A `browser` adapter instead provides an engine to the built-in browser context;
its `list` output uses `name:profile` values and `validate`, `doctor`, and `open`
receive the profile portion as their selection.

A `container` adapter provides an engine to the built-in container family. Its
unqualified `list` output contains native context, connection, or namespace
names; `ctx ls container` prefixes each result as `name:selection`. The provider
may implement `build`, image, and volume capabilities in addition to normal
`run` routing. `ctx image copy` and `ctx volume copy` resolve qualified endpoints
dynamically, so third-party container providers need no core changes.

`selector_key` defaults to the adapter name for selector and container adapters,
and to `browser` for browser adapters. `extra_keys` declares additional profile
values owned by the adapter. `commands` defaults to the adapter name for selector
and container adapters and lets `ctx run` route native command names to the
adapter. `override_env` declares native environment variables, in precedence
order, that `ctx status` should report instead of the stored selection. The
provider remains responsible for honoring them during `run`. `default_provider`
lets legacy unqualified build, image-sync, and
volume endpoints choose a provider without hard-coding an engine in the core;
multiple trusted defaults are reported as an error.

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
~~~

Every declared capability is invoked by the same protocol:

~~~text
ctx-example CAPABILITY SELECTION -- ARGUMENTS...
~~~

Container capabilities currently understood by the core are `build`,
`image_push`, `image_pull`, `image_save`, `image_load`, `volume_exists`,
`volume_create`, `volume_export`, and `volume_import`. Image save arguments are
`ARCHIVE IMAGE...`; image load receives `ARCHIVE`. Volume export writes a tar
stream to stdout, while volume import reads a tar stream from stdin.

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

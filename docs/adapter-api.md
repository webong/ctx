# ctx adapter API v1

An external adapter is a directory containing `adapter.toml` and one executable.
ctx installs adapters under `$CTX_HOME/adapters`; it never discovers or sources
code from the current directory or arbitrary `PATH` entries.

## Manifest

~~~toml
api_version = "1"
name = "example"
kind = "selector"
executable = "ctx-example"
description = "Example context adapter"
capabilities = "list,validate,run,doctor,open"
selector_key = "example_context"
extra_keys = "example_namespace"
commands = "example,examplectl"
first_party = "false"
~~~

Names use lowercase letters, numbers, and underscores, and cannot collide with a
built-in selector or ctx command. `validate` and `doctor` are required. At least
one of `run` and `open` is required. `list` is optional.

`kind` defaults to `selector`. A selector adapter adds a top-level ctx selector.
A `browser` adapter instead provides an engine to the built-in browser context;
its `list` output uses `name:profile` values and `validate`, `doctor`, and `open`
receive the profile portion as their selection.

`selector_key` defaults to the adapter name for selector adapters and `browser`
for browser adapters. `extra_keys` declares additional profile values owned by
the adapter. `commands` defaults to the adapter name for selector adapters and
lets `ctx run` route native command names to the adapter. `first_party` identifies
packages shipped by ctx; it does not bypass checksum trust.

## Process protocol

ctx invokes the executable with one of these forms:

~~~text
ctx-example list
ctx-example validate SELECTION
ctx-example configure SELECTION -- OPTIONS...
ctx-example doctor SELECTION
ctx-example run SELECTION -- ARGUMENTS...
ctx-example open SELECTION -- ARGUMENTS...
~~~

The process receives `CTX_ADAPTER_API`, `CTX_ADAPTER_NAME`,
`CTX_ADAPTER_COMMAND`, `CTX_PROJECT_DIR`, and `CTX_PROFILE`. Values for declared
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
ctx adapter test ./example
ctx adapter install ./example
ctx adapter inspect example
ctx adapter trust example
ctx adapter doctor example
ctx adapter remove example
~~~

Installation does not imply trust. Trust records a checksum over every file in
the adapter directory. Any subsequent change makes the adapter untrusted until the user
reviews and trusts it again.

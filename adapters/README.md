# Adapters

This directory contains every tool-specific integration maintained in the ctx
repository.

External Go adapters can use the public `github.com/webong/ctx/adapter` process
parser and ship a platform-specific `.ctxadapter` archive. A bare ctx binary
installs that archive without Go; source builds use `ctx adapter build`.

`docker`, `podman`, `nerdctl`, and `apple` are maintained virtualizer providers.
Their manifests declare `runtime = "virtualizer"` and `surfaces = "shell"`;
each package owns its native CLI syntax, context discovery, validation, routing,
build behavior, and supported image and volume operations. The ctx core sees
only the common capability protocol.
Docker also implements the versioned `observe` operation for the system graph;
the other providers continue to use their `list` fallback until they expose
additional context or resource observations.

The Docker, Podman, and nerdctl directories also contain tiny command shims. The
installer copies those launcher files into the binary directory under the native
command names so ordinary commands can be context-aware. The provider
implementations themselves remain installed and trusted under
`$CTX_HOME/adapters`.
The `internal/mod` Go package manages these ctx modifications: manifests,
installation, trust, and invocation. Provider behavior stays in `adapters/`.

`firefox`, `chrome`, `chromium`, and `safari` are browser providers. The built-in
`browser` context aggregates them while each provider owns application-specific
profile discovery, validation, launching, and declared browser share operations.
`ctx share:browser` bridges resources between trusted adapters using the
versioned JSON protocol in [the adapter API](../docs/adapter-api.md).
Release and source installers build a separate share executable from each
browser adapter's `native` directory. Shared request handling, policy reading,
and SQLite access live in `internal/app/browser/adapterkit`; Chrome and other
Chromium-based adapters can use the engine owned by `adapters/chromium/engine`.
The executable is part
of that adapter's trusted checksum; CTX core only routes
the declared operation and validates the shared envelope.
The installers update the catalog and refresh active browser adapters with the
packaged helper. Replacing only the `ctx` binary leaves old browser adapter
packages in place; install the new bundle before using browser sharing.

`kube`, `aws`, `gcloud`, `postgres`, and `mysql` are maintained computer-runtime
adapters. Every adapter has an `adapter.toml` manifest and implements ctx adapter
API v2.0. Native installers copy maintained packages into
`$CTX_HOME/catalog/adapters`; `ctx setup` or `ctx adapter add` activates selected
packages under `$CTX_HOME/adapters` and records their checksum trust. A package
can declare `executable_windows` alongside its default executable; the native
core selects the platform implementation at runtime while keeping one manifest,
capability set, and trust record.

Shell-launched AI CLIs use the `computer` runtime and declare
`computer_commands` and optional `computer_capabilities` in their manifest.
Computer integrations can install command shims and provide native hook and
plugin operations. ctx manages the small hook entries that call its bridge;
handlers retain each provider's payload and response contract, and plugin
commands remain provider-specific. See [the adapter API](../docs/adapter-api.md#computer-side-cli-integrations).
The first-party `claude_code` and `codex` packages are bundled in the installer
catalog. Their shims route launches through ctx, hooks forward JSON stdin to
the project-resolved `computer_hook_command` (or the fallback environment
variable `CTX_COMPUTER_HOOK_COMMAND`) with the event name, and plugin operations
delegate to each CLI's native plugin subcommand. `ctx computer hooks install`
writes the CLI's project settings and records the selected handler and events
in the local `.ctx` file.

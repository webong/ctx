# Adapters

This directory contains every tool-specific integration maintained in the ctx
repository.

`docker`, `podman`, `nerdctl`, and `apple` are first-party container providers.
Their manifests use `kind = "container"`; each package owns its native CLI syntax,
context discovery, validation, routing, build behavior, and supported image and
volume operations. The ctx core sees only the common capability protocol.

The Docker, Podman, and nerdctl directories also contain tiny command shims. The
installer copies those launcher files into the binary directory under the native
command names so ordinary commands can be context-aware. The provider
implementations themselves remain installed and trusted under
`$CTX_HOME/adapters`.

`firefox`, `chrome`, `chromium`, and `safari` are browser providers. The built-in
`browser` context aggregates them while each provider owns application-specific
profile discovery, validation, and launching.

`kube`, `aws`, `gcloud`, `postgres`, and `mysql` are maintained selector
adapters. Every adapter has an `adapter.toml` manifest and implements ctx adapter
API v1. Native installers copy maintained packages into
`$CTX_HOME/catalog/adapters`; `ctx setup` or `ctx adapter add` activates selected
packages under `$CTX_HOME/adapters` and records their checksum trust. A package
can declare `executable_windows` alongside its default executable; the native
core selects the platform implementation at runtime while keeping one manifest,
capability set, and trust record.

Shell-launched AI CLIs declare `computer_commands` and optional
`computer_capabilities` in their manifest, with no special adapter kind.
Computer integrations can install command shims and provide native hook and
plugin operations. Hook payloads stream through stdin/stdout; provider config
and plugin formats remain adapter-owned. See [the adapter API](../docs/adapter-api.md#computer-side-cli-integrations).
The first-party `claude_code` and `codex` packages are bundled in the installer
catalog. Their shims route launches through ctx, hooks forward JSON stdin to
`CTX_COMPUTER_HOOK_COMMAND` with the event name, and plugin operations delegate
to each CLI's native plugin subcommand.

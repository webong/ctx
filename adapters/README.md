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

`kube`, `aws`, `gcloud`, `postgres`, and `mysql` are installed first-party
selector adapters. Every installed adapter has an `adapter.toml` manifest and
implements ctx adapter API v1. They are installed under `$CTX_HOME/adapters` and
trusted by the ctx installer. A package can declare `executable_windows` alongside
its default `executable`; the native core selects the platform implementation at
runtime while keeping one manifest, capability set, and trust record.

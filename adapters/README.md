# Adapters

This directory contains every tool-specific integration maintained in the ctx
repository.

`docker`, `podman`, and `nerdctl` are built-in container-engine adapters. Their
executables are transparent command shims, so the installer copies them into the
same binary directory as `ctx` under their native command names.

`firefox`, `chrome`, `chromium`, and `safari` are browser providers. The built-in
`browser` context aggregates them while each provider owns application-specific
profile discovery, validation, and launching.

`kube`, `aws`, `gcloud`, `postgres`, and `mysql` are installed first-party
selector adapters. Every installed adapter has an `adapter.toml` manifest and
implements ctx adapter API v1. They are installed under `$CTX_HOME/adapters` and
trusted by the ctx installer.

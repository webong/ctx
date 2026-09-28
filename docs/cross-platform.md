# Native cross-platform migration

ctx is migrating from its POSIX shell core to a native Go executable. The shell
release remains the default on macOS and Linux until the native implementation
reaches command parity.

## Available in the native preview

- Project `.ctx`, central project map, fallback, and named-profile resolution.
- `ctx status`, `ctx resolve`, `ctx explain`, and `ctx version`.
- Profile environment application through `ctx env`, `ctx run --`, and `ctx shell`.
- Provider-driven Docker, Podman, nerdctl/containerd, and Apple Container
  selection through `ctx run`.
- Native compilation on macOS, Linux, and Windows.
- Windows `.cmd` engine shims and a PowerShell source installer.
- Adapter v1 discovery, checksum trust, lifecycle commands, and process dispatch.
- Selector adapters for Kubernetes, cloud CLIs, and databases.
- Browser-provider aggregation, validation, launching, and diagnostics.
- Atomic, lock-protected `set`, `clear`, and profile configuration mutations.
- Registry-backed build caches plus image and named-volume transfers across
  Docker, Podman, nerdctl/containerd, and Apple Container endpoints.
- Windows-native PowerShell providers for the bundled browser, Kubernetes, cloud,
  PostgreSQL, and MySQL adapters.
- Checksum-verified macOS, Linux, and Windows release bundles with GitHub build
  provenance attestations and source installers for each platform.
- Explicit `CTX_SHELL`/`--shell` selection and dynamic PowerShell completion for
  commands, adapters, profiles, selectors, and discovered context values.
- A cross-platform adapter catalog and `ctx setup` selection flow, with
  non-interactive `--adapters`, `--all`, and `--minimal` installation modes.

Build the preview locally:

~~~sh
go build -o ./ctx-native ./cmd/ctx
./ctx-native version
~~~

On Windows, from a source checkout:

~~~powershell
.\install.ps1
ctx.exe version
~~~

## Release readiness

The intended 0.8 native command surface is implemented. The remaining work is
release-candidate soak testing on real Windows, macOS, and Linux setups before
making the native executable the default installer target.

The target architecture keeps `shell`, `browser`, and `container` as ctx context
families. Individual applications are first-party or external providers, so
operating-system support and CLI-specific behavior live in provider packages
rather than accumulating platform or engine switches in the core. Container
providers are discovered from installed `kind = "container"` adapters; adding one
does not require recompiling ctx.

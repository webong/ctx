# Cross-platform support

ctx uses a native Go core on macOS, Linux, and Windows. Tool-specific behavior
lives in adapter packages, with POSIX implementations for macOS and Linux and
PowerShell implementations where Windows needs them.

## Supported capabilities

- Project `.ctx`, central project map, fallback, and named-profile resolution.
- `ctx status`, `ctx resolve`, `ctx explain`, and `ctx version`.
- Profile environment application through `ctx env`, `ctx run --`, and `ctx shell`.
- Provider-driven Docker, Podman, nerdctl/containerd, and Apple Container
  selection through `ctx run`.
- Native compilation on macOS, Linux, and Windows.
- Windows `.cmd` engine shims and a PowerShell source installer.
- Adapter API v2.0 discovery, checksum trust, lifecycle commands, and process dispatch.
- Runtime-independent `supports` declarations for resource kinds such as
  virtualizers and containers, projected into the system graph.
- Platform-specific `.ctxadapter` archives and checksum-pinned indexes, so a bare
  ctx binary can install compiled adapters without a Go toolchain.
- Computer-runtime adapters for Kubernetes, cloud CLIs, and databases.
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

Build ctx locally:

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

The intended 0.8 command surface is implemented and the native executable is the
default installer target. Release candidates must still pass real-machine soak
testing on Windows, macOS, and Linux before a stable tag is published.

The architecture separates three runtimes—`computer`, `manager`, and
`browser`—from two interaction surfaces: `shell` and `web`. Individual
applications are maintained or external providers, so operating-system support
and CLI-specific behavior live in adapter packages rather than accumulating
platform or engine switches in the core. Providers are discovered from their
API v2.0 `runtime`, `surfaces`, and `capabilities`; adding one does not require
recompiling ctx.

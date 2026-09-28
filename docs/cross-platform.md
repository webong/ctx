# Native cross-platform migration

ctx is migrating from its POSIX shell core to a native Go executable. The shell
release remains the default on macOS and Linux until the native implementation
reaches command parity.

## Available in the native preview

- Project `.ctx`, central project map, fallback, and named-profile resolution.
- `ctx status`, `ctx resolve`, `ctx explain`, and `ctx version`.
- Profile environment application through `ctx env`, `ctx run --`, and `ctx shell`.
- Docker, Podman, and nerdctl/containerd selection through `ctx run`.
- Native compilation on macOS, Linux, and Windows.
- Windows `.cmd` engine shims and a PowerShell source installer.

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

## Remaining parity work

- Adapter discovery, trust, and process execution in Go.
- Browser-provider discovery and Windows browser implementations.
- `set`, `clear`, and profile mutation commands.
- Build-cache, image, and volume transfer commands.
- Native macOS/Linux installation and signed release artifacts.
- PowerShell completion and richer Windows shell selection.

The target architecture keeps `shell`, `browser`, and `container` as ctx context
families. Individual applications remain providers, so operating-system support
belongs in the provider rather than accumulating platform switches in the core.

# Host discovery

Host inventory belongs to the public `github.com/webong/ctx/graph/system`
package. It works in a standalone CTX installation without adapter packages.
Adapter inventory describes provider capabilities and contexts separately.

```sh
ctx graph shells
ctx graph filesystems
ctx graph webviews
ctx graph scan
ctx graph vertices shell
ctx graph vertices filesystem
ctx graph vertices webview
ctx graph edges has-shell
ctx graph edges has-filesystem
ctx graph edges has-webview
```

`shells`, `filesystems`, and `webviews` discover the host, emit JSON, and refresh
only their corresponding graph records. `scan` refreshes all host inventories and adapter
inventory. Removed executables and mounts are removed from the next observation.
Stored snapshots are observations, not authorization or a guarantee that an
executable or mount will still be available when another command uses it.

## Shells

Discovery records each available executable path separately, its resolved
symlink target, and the sources that identified it. It checks executable files
and Unix execute permissions without starting the shell or reading startup
scripts or history.

On macOS and Linux, sources include `/etc/shells`, standard shell names across
absolute PATH entries, and the executable declared by `SHELL`. Registered or
configured shells can have arbitrary names. PATH discovery recognizes standard
names including Bash, Zsh, Fish, PowerShell, Nushell, Elvish, Xonsh, and common
POSIX shells; it cannot identify an arbitrarily named executable as a shell.

On Windows, discovery includes standard shell executables on PATH, `ComSpec`,
the system Command Prompt and Windows PowerShell locations, and installed
PowerShell versions under Program Files. This does not inspect shells inside
WSL distributions or remote machines.

`configured_default` reflects `SHELL` or `ComSpec`. It does not claim to identify
the process actually invoking CTX. Existing `shell-session` records continue to
describe the session separately. Versions are not collected by executing tools.

## Mounted filesystems

- macOS uses the native mount table and its cached space statistics.
- Linux uses `/proc/self/mountinfo` and filesystem statistics, including escaped
  mount paths, bind mounts, and the calling process's mount namespace.
- Windows enumerates volume mount paths, including mounted folders, and logical
  drives including mapped network drives. Removable drives without media are
  omitted. Filesystem names, volume labels, read-only status, and disk space come
  from native Windows APIs.

Each entry includes its mount point, source, filesystem type and read-only
status. `space` contains `total_bytes`, `free_bytes`, and `available_bytes` when
the OS provides them. Available bytes reflect the calling user's ability to
allocate space. Windows total bytes can also reflect a user quota. Missing
space or volume details produce `detail_error`; absent type details also mean
read-only status could not be determined. Network-source URL user information
is removed from output and stored records. Mount options and file contents are
not collected.

This lists mounted filesystems visible to CTX, including pseudo-filesystems on
Linux. It does not list unmounted disks, every filesystem format the OS could
support, or mounts outside a container's visible namespace. Statistics are a
point-in-time view; shared storage pools must not be summed as independent disks.
Network mount queries can depend on the availability of the remote server.

## Shared web engines and webviews

`ctx graph webviews` inventories web embedding runtimes independently of CTX
adapters. Each record identifies its name, rendering engine (`webkit` or
`blink` for the built-in collectors), embedding API, scope, source, and evidence
location. Runtime versions, library ABI generations, resolved paths, and ELF
architecture are included where available. An ABI generation is not a WebKit
or Chromium release version.

| Platform | Discovery | Evidence |
| --- | --- | --- |
| macOS | System WebKit / WKWebView | System framework and its bundle version; supports frameworks whose binaries live in the dyld shared cache |
| Windows | Evergreen WebView2 / Blink | Microsoft's version registration in machine and user registry locations, across both registry views |
| Linux | Shared WebKitGTK / WebKit and Qt WebEngine / Blink | ELF shared libraries in standard, multiarch, `ld.so.conf`, and absolute `LD_LIBRARY_PATH` directories |

Linux recognizes WebKitGTK API generations 4.0, 4.1, and 6.0, and Qt WebEngine
Core generations 5 and 6. Aliases of one library are reported once; distinct
libraries and ABI generations remain separate. Architecture is the ELF machine
identifier. Windows reports one entry per scope/version when registry views
repeat a registration; a registry view does not establish runtime architecture.
No WebView2 registration produces an empty inventory rather than assuming that
Edge or Chrome provides a shared runtime.

These are installation observations. Discovery does not load native libraries,
create a webview, validate dependent libraries or GUI availability, or promise
that an arbitrary app can use a runtime. Windows registry evidence can be stale.
macOS version metadata is read with the system `plutil` utility; unavailable
metadata retains the framework observation with `detail_error`.

App-private Electron/CEF/Chromium bundles, fixed-version WebView2 bundles,
arbitrary Qt SDK installations, sandbox-private runtimes, and remote/mobile
devices are outside this host scan. An installed browser alone does not count
as a shared engine. Cookies, sessions, policies, profiles, and permissions remain
owned by each embedding app; engine inventory provides no access to them.
App-specific observations and operations can be supplied by their adapters,
and library consumers can project additional runtimes through `ObserveHost`.

Reference contracts: [Apple WKWebView](https://developer.apple.com/documentation/webkit/wkwebview/),
[Microsoft WebView2 runtime detection](https://learn.microsoft.com/en-us/microsoft-edge/webview2/concepts/distribution),
[WebKitGTK API generations](https://webkitgtk.org/reference/webkit2gtk/2.39.1/migrating-to-webkitgtk-6.0.html),
and [Qt WebEngine](https://doc.qt.io/qt-6/qtwebengine-overview.html).

## Library API

Use discovery without a store, or persist the result in an existing system
graph:

```go
inventory, err := systemgraph.DiscoverHost(ctx)
// inventory.Shells, inventory.Filesystems, and inventory.Webviews are typed observations.

inventory, err = system.ScanHost(ctx)
// ScanHost discovers and atomically projects all host inventories into the graph.
```

`DiscoverShells`, `DiscoverFilesystems`, and `DiscoverWebviews` can be called
independently.
`Graph.ObserveHost` accepts a `HostInventory`; a nil slice preserves that part
of the existing inventory, while a non-nil empty slice removes its old entries.
The schema uses `shell`, `filesystem`, `webview`, and `host-inventory` vertices
linked to `machine/local`. The generic graph store owns persistence and transactions;
`graph/system` owns host discovery and its vocabulary.

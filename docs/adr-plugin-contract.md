# ADR: Shared plugin contract

Status: implemented initial v1 contract in the CTX library. Consumer migrations
to the JSON binding require explicit integration; existing Xallet and Cymonkey
wire protocols do not become compatible merely by importing this package.

## Ownership

CTX supplies `github.com/webong/ctx/plugin` and
`github.com/webong/ctx/plugin/jsonline`, alongside `graph` and `supervisor`.
The plugin library owns generic declarations, validation, exact selection,
admission sequencing, handshake checking, bounded invocation, and draining.
It has no product catalog, default installation location, native command map,
browser fallback, or built-in permission vocabulary.

Consumers define domain contracts, payloads, authorization, configuration,
discovery, package distribution, trust storage, and endpoint authentication.
Product and native mechanisms in CTX belong under `adapters/<name>/`.
Portable integrity and transport utilities may be shared. A descriptor, graph
record, matching checksum, or successful handshake never grants authority.

## One contract, independent versions

There are three independent identities:

1. `apiVersion: ctx.plugin/v1` specifies the shared envelope and semantics.
2. `identity` pins a package ID, immutable revision, and optional release
   version. A revision must change with any selected artifact or configuration
   change, including changes that keep the same release version.
3. Each contract has a consumer-owned name and exact version. The library does
   not infer compatibility from semantic version ranges or choose a provider
   by name. Consumers resolve release preferences before selection.

`Descriptor` declares 1–64 unique contract references, each with 1–256 unique
operations. An operation may name a consumer-defined surface for admission.
For example, Xallet may use `observation` and `action`; CTX does not assign
permissions to those names. Capability and role catalogs remain domain data.

`Select` requires a contract and operation, optionally pins the complete
identity, and rejects zero or multiple matches. Declaration order has no
selection authority. Returned descriptors and session snapshots are copies.

`MatchHandshake` requires the selected identity and complete contract set,
including operations and surfaces, to match exactly. Ordering is insignificant.
Both capability expansion and reduction fail. This binds claims to a reviewed
selection; the transport and process owner must establish which peer made them.

## Host admission and lifecycle

`Open` validates the selected descriptor, calls the required `Verify` callback,
opens the caller-selected backend, and checks its handshake. A missing verifier
or authorizer fails closed. Verification must bind the selection to the actual
installed content and the consumer's trust policy before launch or connection.
`DirectoryDigest` provides portable integrity checking of a package directory,
including manifests and assets. It does not establish publisher authenticity.

`Session.Call` supplies the identity, correlation ID, selected surface, and
deadline. It validates the request and calls `Authorize` for every invocation
before dispatch. Policy callbacks receive copies. Domain-specific identity,
approval references, scope, and intent belong in the domain payload or a
consumer-owned backend. Credential values should use an explicit secret
delivery mechanism rather than ordinary invocation payloads. The library
cannot recognize arbitrary secret bytes and never logs payloads.

Sessions enter `ready` after admission and handshake. `Close(ctx)` changes to
`draining`, rejects new calls, waits for admitted calls, and closes the backend.
A drain deadline restores admission without closing the backend. Concurrent
close attempts receive `ErrDraining`. `Abort` immediately closes the transport.
Transport or response-integrity failures make the session `failed` and close
the backend; structured domain errors leave it usable. Calls are never retried
automatically, because a timed-out operation may already have taken effect.

The default handshake, call, and drain timeout is 30 seconds. `Options.Timeout`
sets the session bound; an earlier caller deadline wins. Backend implementations
must honor context cancellation and make `Close` interrupt outstanding I/O.

Process lifecycle remains in `supervisor`: leases, process trees, health,
restart policy, secret resolution, resource controls, and graph projection.
An embedding host starts an already-authorized `supervisor.Spec`, uses
`EndpointReady` for its declared endpoint, and opens a session in
`ProtocolHandshake`. It drains the session before `Supervisor.Stop`, and uses
`Abort` on process failure or forced shutdown. Each restart needs a new session
and fresh handshake. The host must fence old session references when replacing
a process and own rollback of a multi-plugin composition. The plugin library
does not implement an additional process supervisor or composition database.

## JSON-line binding

`jsonline.NewClient` and `jsonline.Serve` operate on a supplied
`io.ReadWriteCloser`. This supports local IPC or process pipes supplied by the
host; the binding does not discover endpoints, launch commands, or authenticate
network peers. Closing the connection must unblock its reads and writes.

Messages are single UTF-8 JSON values followed by LF, with a maximum JSON frame
size of 24 MiB (excluding LF). Unknown envelope fields, duplicate JSON keys,
trailing values, and nesting deeper than 64 levels are rejected. Payload fields
are opaque JSON governed by the consumer's contract. Protocol stdout must be
kept separate from diagnostic stderr when used over process pipes.

The first request uses `operation: plugin.hello`, `id: hello`, the shared API
version, empty plugin/contract identities, and a deadline. Its response payload
is the descriptor. Subsequent requests carry the selected identity, exact
contract reference, operation, declared surface, deadline, and optional payload.
The server validates the envelope before calling its domain handler. The server
handler is responsible for its own consumer authorization and must honor its
context. The supplied server caps each operation at 30 seconds by default;
`jsonline.Server.MaxCallDuration` configures this bound independently of caller
deadlines. The client uses the earlier host/request deadline, or 30 seconds when
neither has one.

Every response echoes the request ID and API version, with exactly one of
`payload` or `error`. A successful empty result uses JSON `null`. Errors have a
stable `code`, bounded `message`, and optional nonnegative
`retryAfterMilliseconds`. Domain errors returned as `plugin.RemoteError` retain
their category. Other server-handler failures become `operation_failed`, with
generic text. Error text from a plugin remains untrusted diagnostic data.

Calls on one JSON client are serialized. Cancellation while waiting for the
transport gate is bounded; cancellation during I/O closes the connection.
A session treats backend cancellation as terminal and requires a fresh session.
Closing the transport does not imply termination of a separately supervised
process. There is no automatic replay, idempotency, streaming-event channel,
server-initiated callback, or reconnect in v1. Domains can define bounded polling
methods and use separately declared channels for streams.

## CTX adapter adoption

`adapter.PluginDescriptor` maps existing adapters to `ctx.adapter@2.0`.
The internal binding preserves legacy adapters' declared native API versions.
Capabilities, computer hook/plugin capabilities, and declared browser management
support become operations. Installed identity is `ctx.adapter/<name>` with a
`sha256:<directory digest>` revision.

The CTX adapter loader's capability queries use the shared contract lookup.
Trust checks validate the shared descriptor and use the shared directory
digest implementation. The sorted path/file-hash format is unchanged, so
existing trust records remain usable. Special files and paths with line breaks
are rejected along with symlinks.

The argv/environment/stdio adapter protocol is an explicit compatibility
binding. CTX shell adapters can carry interactive or binary streams without
wrapping those streams in JSON. Their package manifest and checksum provide
the selected descriptor; they do not pretend to perform a JSON handshake.
Command execution retains existing trust checks and invocation behavior.
Native commands such as `ctx plugin computer <adapter>` still invoke the
selected adapter's product-specific plugin operation.

## Consumer mapping

| Consumer | Shared mechanics | Consumer-owned contract and policy |
| --- | --- | --- |
| CTX adapters | Descriptor validation, operation lookup, directory integrity | `ctx.adapter`, argv/stream binding, context selection, native behavior in adapters |
| Xallet Package plugins | Selection, handshake, admission sequencing, call envelopes, draining | Roles, Package/Node/Worker identities, surfaces, Host Broker permits, approval, secrets, reconciliation |
| Xallet platform extensions | The same host mechanics where applicable | Contribution points, route mounting, schema restrictions, generations and composition rollback |
| Cymonkey plugins | Selection, verified package identity, bounded transport, structured errors | Display/content/device methods, domain adapters, device grants, runtime choices |

Xallet's `xallet.plugin.v1` and platform-extension envelopes can be translated
by a consumer-owned `plugin.Backend` while implementations migrate. Cymonkey's
existing `plugin.hello` and `provider.plugin/v1alpha1` packages likewise need an
explicit translation or a new native binding; sharing a hello method name does
not make the envelopes compatible. The generic library must not detect either
product by name to choose a translation.

Browser augmentation source packages, userscript approval, extension signing,
native installation, and UI contribution semantics remain in their owning
domain or adapter. A common package descriptor can declare those capabilities
without moving their implementation into the plugin core.

## Compatibility and validation

Treat the v1 wire fields, exact matching rules, and failure behavior as a public
contract. Unknown fields fail strict decoding; protocol additions that require
new wire fields need a separately supported API version. New domain methods
can be declared without changing the generic protocol, and independently
versioned domain contracts may coexist in a descriptor.

Run the end-to-end example with `go run ./examples/plugin`. Contract, admission,
drain, integrity, malformed-frame, cancellation, concurrency, and CTX compatibility
tests live under `plugin/`, `plugin/jsonline/`, and `internal/mod/plugin_test.go`.

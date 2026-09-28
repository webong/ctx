# ADR: Generic graph storage and process supervision

Status: accepted for the CTX library.

## Ownership boundary

CTX supplies reusable mechanics through the public `github.com/webong/ctx/graph`
and `github.com/webong/ctx/supervisor` packages. CTX graph records are generic
vertices and directed edges. A consumer registers a namespace and schema version,
supplies an optional validation callback, and owns its kind and relationship
vocabulary. Namespace-qualified names use the form `<namespace>/<name>`.

CTX does not assign meaning, authority, permission, trust, or membership to a
record or relationship. A schema callback may reject a proposed transaction
based on a transaction-scoped read-only view. It must not be used to create a
second authorization system inside CTX. Cycles are allowed unless a consumer
validator rejects them.

The `supervisor` package owns local process mechanics. Callers decide why a
process may run and pass an already-approved `Spec`. The supervisor starts,
connects, observes, restarts, and stops that process. Its runtime facts describe
observations and never imply consumer authority.

## Graph API contract

Consumers call `Register(Schema)` before use, then `Apply(Transaction)` to make
an atomic change. All records in a transaction belong to one namespace. Vertices
must exist before edges can reference them; node and edge creation can happen in
the same transaction. An invalid endpoint, name, secret-like field, or consumer
constraint rejects the whole transaction without advancing its revision or
publishing a change.

IDs are caller supplied and stable. Repeated IDs upsert records. `CreatedAt` is
preserved on update and `UpdatedAt` plus `Revision` identify the commit that
last changed the record. `ExpectedRevision` compares against that namespace's
current revision. `IdempotencyKey` is scoped to a namespace and binds to the
canonical JSON form of the transaction: replaying the same request returns its
original commit cursor; reusing the key with a different request is an error.

Edges cannot dangle. Deleting a vertex with incident edges fails unless the
transaction sets `DeleteIncidentEdges`, which explicitly detaches all incident
edges. Deleting records and adding replacements in the same transaction is
supported. Each successful transaction advances one namespace revision and one
store-wide monotonically increasing change cursor.

Queries require a positive result limit. Traversal is breadth first, follows
outgoing edges, and takes explicit maximum depth and vertex bounds. Results are
deterministically ordered by IDs where ordering is not traversal order. A
snapshot returns a consistent namespace view and its revision.

`Watch(ctx, namespace, afterCursor, buffer)` replays retained changes newer than
the cursor and then streams later commits in cursor order. An empty namespace
watches all namespaces. If its bounded channel fills, the stream closes; the
consumer should reconnect from the last cursor it processed. This avoids
unbounded memory growth and silent reordering. The current file backend retains
its change history in the snapshot; deployments should compact or rotate the
file as their event retention needs evolve.

## Persistence and limits

`graph.NewMemory` is the reference implementation. `graph.OpenFile` stores the
complete state as JSON and commits by writing a mode-0600 temporary file followed
by atomic rename while holding an OS file lock. Every operation refreshes from
the latest committed snapshot under that lock, so short-lived CTX commands and
long-lived local consumers do not overwrite each other's commits. File-backed
watches poll the durable cursor every 150 ms and publish changes in order.
Callers register their schema callback each time a store is opened; the durable
file remembers the namespace schema version and rejects a version mismatch.
The backend does not provide replication or partial history compaction.

The current implementation uses in-memory maps and bounded result slices, and
rewrites the full JSON snapshot on every commit. It does not provide a
distributed index or an unbounded query API. Large graph installations should
use a backend with indexed persistence behind the same interface.

## CTX system context

CTX's integration layer owns the `ctx.system` namespace. Short-lived CTX
commands record the invoking shell session, current directory, detected project,
active profile, and selected adapter context IDs. Optional Bash, Zsh, and
PowerShell prompt hooks refresh shell location after directory changes.
Successful browser opens add the provider, profile, and URL origin. URL paths,
query strings, fragments, arbitrary environment values, and shell history are
not collected. `ctx graph` is a short-lived inspection and export interface over
the persistent store. Other services register separate namespaces and cannot
change CTX's schema rules.

## Security

Graph records may contain ordinary metadata only. Common secret-bearing field
names (including password, token, credential, private key, and API key) and
explicit `secret-value:` / `private-key:` markers are rejected in attributes and
labels. This catches accidental credential serialization; it cannot identify
arbitrary secret bytes hidden in a generically named string. Consumers must
store only opaque scoped references and resolve them at the point of use.

`supervisor.Spec.Environment` contains ordinary process configuration.
`SecretReferences` maps environment names to opaque `{scope, id}` references;
an injected resolver materializes values only for the child environment. The
resolver result is never returned in an `Instance`, log record, or graph fact.
The supervisor projects artifact ID/revision/checksum, process state, health,
crashes, runtime identity, endpoint and connection metadata, and generic relationships. Logs
are an in-memory tail bounded by `Options.LogLimit`.

## Consumer integration: Xallet

Xallet should own and register `xallet.io` schema version `1`, including its
Package, Pack, Packet kinds, its exact relationship vocabulary, and all semantic
chain constraints. Its validator should enforce its own role/type combinations
using the proposed transaction and `View`. It should keep authorization and all
other product decisions outside CTX. See `examples/xallet-shaped` for a small
consumer-owned example.

Xallet's adapter should:

1. Open a `graph.Store` (initially `graph.OpenFile` or `graph.NewMemory`) and
   register its own schema version and validator.
2. Translate Xallet-owned entities to stable vertex IDs and namespace-qualified
   kinds; translate relationships to namespace-qualified edge types.
3. Apply related changes in one `Transaction`, setting `ExpectedRevision` for
   concurrent writers and a stable `IdempotencyKey` for retried requests.
4. Use namespace-scoped bounded queries, `Snapshot`, and `Watch` cursors for
   reads and projections; revalidate/authorize in Xallet at the product boundary.
5. Start only processes Xallet has already authorized. Translate selected
   binary identity/revision/checksum, ordinary environment, opaque secret
   references, endpoints, and dependencies to `supervisor.Spec`; consume
   lifecycle and health observations without interpreting them as permission.
6. Keep Xallet IDs, roles, vocabulary, validation, graph migration, trust,
   approvals, and authorization out of CTX core.

CTX has no database migrations to run for Xallet. Xallet needs a bootstrap and
migration layer for its schema version, an adapter that maps its existing
product records to/from graph snapshots and transactions, a policy validator,
and a supervisor bridge for approved Package binaries. Existing Xallet storage
remains authoritative until that adapter is deliberately switched over.

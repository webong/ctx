# HashiCorp implementation of the CTX plugin library

`github.com/webong/ctx/plugin/hashicorp` supports hosts and guests using
[HashiCorp go-plugin](https://github.com/hashicorp/go-plugin). It is a backend
of the public plugin library, alongside `plugin/jsonline`. CTX adapters,
Xallet, Cymonkey, and other applications may consume either implementation.

The shared `plugin.Endpoint`, `plugin.Guest`, `plugin.Backend`, and
`plugin.Session` contracts are transport-independent. This backend supplies
the native go-plugin launch handshake and net/rpc or gRPC client/server bridge.

## Guest

Create a `plugin.Guest` with your descriptor and domain handler, then register
it in a native go-plugin server:

```go
guest, err := plugin.NewGuest(descriptor, plugin.GuestOptions{
    Handler: authorizeAndHandle,
})
if err != nil {
    return err
}
goplugin.Serve(&goplugin.ServeConfig{
    HandshakeConfig: hashicorp.HandshakeConfig(),
    Plugins: goplugin.PluginSet{
        hashicorp.PluginName: &hashicorp.Plugin{Guest: guest},
    },
    GRPCServer: hashicorp.GRPCServer,
})
```

Omit `GRPCServer` to serve net/rpc. The handler checks domain authorization,
honors its context, and supports concurrent calls. Both transports share the
same CTX descriptor and request/response validation.

## Host

Use `plugin.Open` with the consumer's trust verifier and per-call authorizer.
Inside its `Connect` callback, create a dedicated `goplugin.Client` with the
same native handshake and `hashicorp.Plugin{}` in its `Plugins` map, then call
`hashicorp.Connect(ctx, client)`. The supplied runnable example shows the full
configuration, including executable checksums and automatic TLS.

Set `AllowedProtocols` explicitly to `ProtocolGRPC` or `ProtocolNetRPC`, and
bound `StartTimeout` to the remaining connection deadline. Configure process
environment and logging deliberately: go-plugin's defaults are not CTX's
domain policy. The native magic cookie is not an authorization credential.

`Connect` owns the client once called. Successful backends kill the child on
session shutdown; failures clean up the client. Cancellation during native
startup returns promptly and cleans up after native startup finishes or times
out. The child has one lifecycle owner: this go-plugin client. Do not also
start or stop the same child through CTX supervisor.

Direct users of go-plugin's `Dispense` receive a `plugin.Backend` and keep
responsibility for killing their own client. Closing a gRPC binding cancels
its own calls without closing a shared gRPC connection. net/rpc closes the
dispensed interface's dedicated stream. For managed ownership use `Connect`.

## Existing plugin implementations

`ConnectInterface(ctx, client, name, bind)` accepts an existing native plugin
name and translates its dispensed Go interface through `bind`. The translation
supplies CTX's descriptor and invocation semantics, including cancellation.
Native interfaces are not automatically interchangeable: each domain mapping
must be explicit. No core provider-name switch chooses a translation.

Future plugin implementations can add their own library backend against
`plugin.Backend` and `plugin.Endpoint` without changing the host API.

## Wire format and limits

[`runtime.proto`](runtime.proto) defines `ctx.plugin.v1.Runtime` with unary
`Handshake` and `Invoke` methods. Standard protobuf `BytesValue.value` carries
the UTF-8 CTX JSON descriptor or envelope, without LF. The manual Go service
registration uses the standard protobuf codec and well-known message types;
other languages can generate stubs directly from the service definition.
Non-Go processes must also implement the upstream go-plugin startup handshake
and the configured transport authentication; the schema describes the CTX
service inside that runtime.

The supplied gRPC server factory bounds protobuf messages, and both sides
check the shared 24 MiB JSON limit. net/rpc's upstream Gob decoder allocates
before the JSON bound is checked. CTX's request IDs provide correlation, not
automatic replay or exactly-once execution. A canceled call may have taken
effect. HashiCorp broker callbacks and streaming require explicit domain
bindings; the standard CTX interface remains unary.

## Run

```sh
go run ./examples/plugin-hashicorp --protocol grpc
go run ./examples/plugin-hashicorp --protocol netrpc
go test -race ./plugin/...
```

The example launches its own reviewed executable as the guest. Tests launch
real subprocesses and need permission to create local IPC sockets.

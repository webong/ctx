# Adapter ownership

Anything specific to a product, provider, browser, or native tool belongs in its
adapter package under `adapters/<name>/`. This includes native protocols,
installation and activation, executable and profile discovery conventions,
storage formats, signing tools, store URLs, settings paths, and diagnostics.

CTX core owns generic contracts, validation, trust, selection, and dispatch.
Shared libraries may implement portable workflows and utilities, but must not
choose native behavior by adapter name or supply a browser-specific fallback.
Adapters provide that behavior through declared capabilities and backends.

Related adapters may reuse an engine from the adapter that owns the native
mechanism, such as `adapters/chromium/engine`. Each product adapter supplies its
own paths, identifiers, capabilities, and policy through configuration.

Apply this boundary when bringing code into CTX: move provider-specific behavior
into the owning adapter before wiring it into shared workflows.

# Public libraries

`graph` and `plugin` are reusable libraries for CTX and its ecosystem. CTX
adapters, Xallet, and Cymonkey can build on them as consumers. Plugin runtime
backends, including HashiCorp go-plugin, belong inside the plugin library
(for example `plugin/hashicorp`), together with their dependencies. They are
not CTX product adapters. Keep the shared host/guest contract independent of
backend choice so existing and future plugin implementations can use it.

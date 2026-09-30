# Browser profile management

CTX exposes reusable profile-scoped workflows for browser extensions,
userscripts, and bookmarklets. A trusted browser adapter owns the
browser-specific operations. The same adapter can keep display, page, and
session operations behind a separate runtime and implement the public CTX
backend interface as a bridge to that runtime.

## CLI

Choose a profile explicitly or select it with `ctx set browser <adapter>:<profile>`:

~~~sh
ctx browser manage extension targets --target firefox:Profile\ 1
ctx browser manage extension prepare --target chrome:Default \
  --input '{"source":"/path/to/extension.zip"}'
ctx browser manage extension install --target firefox:Profile\ 1 \
  --input '{"source":"/path/to/signed-extension.xpi","revision":"sha256:…"}'
ctx browser manage extension activate --target chrome:Profile\ 1 \
  --input '{"source":"/path/to/extension.zip","revision":"sha256:…"}'
ctx browser manage userscript prepare --target firefox:Profile\ 1 \
  --input '{"source":"/path/to/script.user.js"}'
ctx browser manage bookmarklet encode --target firefox:Profile\ 1 \
  --input '{"source":"/path/to/bookmarklet.js"}'
~~~

The adapter must declare each operation it implements in
`browser_management = "kind.action,..."`. CTX rejects undeclared operations,
checks adapter trust, and validates protocol version, response identity, and
the separation between extension installation and session activation. Use
`--input-file PATH` or `--input-file -` for structured or larger JSON input.

## Operations

`extension` supports `targets`, `capabilities`, `prepare`, `package`, `sign`,
`stage`, `install`, `activate`, `store_install`, and `store_remove`.
Preparation inspects the package and produces a revision for review. Packaging
and signing produce artifacts. Staging copies files for a later browser action.
Installation uses the selected browser's native or guided install route.
Activation loads an extension into a controlled session and must not claim a
persistent installation. Store requests only register a browser-native request;
they are not proof that an extension is installed or enabled.

`userscript` supports `prepare`, `install`, `update`, `list`, `describe`,
`enable`, `disable`, `uninstall`, and `activate`. `bookmarklet` supports
`encode`, `decode`, and `install_page`. A bookmarklet is exported for the user
to save and click; encoding it does not execute it or edit browser bookmarks.

The request is a JSON object with `version`, `kind`, `action`, and optional
operation-specific `input`. The response has `version`, `kind`, `action`,
`status`, and an optional JSON `result`. Input and response sizes are bounded.
Status values remain adapter-defined, subject to the install/activation
boundary validated by CTX.

## Go adapter API

`github.com/webong/ctx/adapter/browser` exposes the request and response types,
operation catalog, validators, `ManagementBackend`, and `RunManagement`.
Implement `ManageBrowser` to connect the protocol to browser-specific logic:

~~~go
type managementBackend struct{}

func (managementBackend) ManageBrowser(
    ctx context.Context,
    profile string,
    request browser.ManagementRequest,
) (browser.ManagementResponse, error) {
    // Dispatch an operation to the browser provider and its page/session runtime.
    result := browser.ManagementResult(map[string]string{"profile": profile})
    return browser.ManagementResponse{
        Version: browser.ManagementVersion,
        Kind: request.Kind,
        Action: request.Action,
        Status: "prepared",
        Result: result,
    }, nil
}

func main() {
    invocation, err := adapter.Parse(os.Args[1:])
    if err != nil { os.Exit(2) }
    if invocation.Operation == "manage" {
        os.Exit(browser.RunManagement(context.Background(), invocation.Selection, os.Stdin, os.Stdout, os.Stderr, managementBackend{}))
    }
}
~~~

The same package exposes `PageSessionRuntime` and `PageSession` for explicit
target discovery, page navigation, script injection, and userscript replay.
The provider can call its page/session runtime through this interface while
keeping CDP, WebDriver BiDi, and replay behavior behind that implementation.
The current adapter runner dispatches the versioned request from stdin. The
provider must preserve native browser consent and policy requirements, verify
package revisions before install or activation, and report persistence only
when its native route verifies it. Callers can add review, approval, and audit
around this reusable API without making their higher-level policy part of CTX.

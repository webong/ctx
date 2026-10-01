# Cookie inputs and reliable automation

Cookie storage, profile discovery, credentials, and native scope mapping belong
to browser adapters. CTX supplies portable validation, source selection, and the
bridge between files, processes, and profiles. The public `browser.Get` API uses
the same trusted adapters as `ctx share:browser cookie query`.

## Accepted input

Query inline/fallback inputs and `cookie import` accept:

- A version-2 ctx single-cookie bundle from `cookie copy`.
- A ctx query result with a `cookies` array, or a standalone cookie array.
- A Netscape cookie jar with seven tab-separated fields. The subdomain flag,
  Secure flag, expiry, empty values, and `#HttpOnly_` prefix are preserved.
- JSON cookies using `httpOnly`, `hostOnly`, `sameSite`, and `expirationDate`
  or `expires` aliases for the portable fields. Fractional expiry is rounded
  down to seconds; `expires: -1` denotes a session cookie. `hostOnly` determines
  whether the domain allows subdomains. `no_restriction` maps to SameSite `none`.

Inputs must be UTF-8 and at most 8 MiB. Parsing rejects missing values,
conflicting aliases, malformed rows, control characters, and unsupported
fields. Errors identify fields or row numbers without including cookie values.
Native `storeId`, `firstPartyDomain`, and structured `partitionKey` fields need
adapter normalization when populated: the generic parser will not discard
their scope. CTX's `partition_key` and namespaced `attributes` are preserved.
Netscape jars cannot carry SameSite or partition/container scope; use ctx JSON
when that information is needed. See the [curl jar format](https://curl.se/docs/http-cookies.html)
and [JSON cookie field definitions](https://developer.mozilla.org/en-US/docs/Mozilla/Add-ons/WebExtensions/API/cookies/Cookie).

Other Go services can use `browser.ParseCookies(data)` to normalize these inputs
and `browser.BundleCookie(cookie, site)` to produce one validated import bundle.
`InlineCookies.Data`, `JSON`, `Base64`, and `File` are mutually exclusive ways to
supply input. `FallbackInline` uses the same formats.

## Import a selected cookie

```sh
ctx share:browser cookie import --from-file ./site-cookies.json \
  --site https://example.com --name session --to-profile chrome:Default

ctx share:browser cookie import --from-file ./cookies.txt \
  --site https://example.com --name session --domain .example.com \
  --path /account --to-profile chromium:Default
```

A ctx bundle supplies its site. Other inputs require `--site`. Import selects
exactly one active cookie; ambiguous matches require `--domain`, `--path`, or
namespaced `--attribute key=value` filters. It preserves the source label when
available, validates secure prefixes and SameSite, and sends a version-2 bundle
to the target adapter. This does not perform a bulk import. An input query's
warnings are reported before importing the selected cookie.

The destination must declare `cookie.import` and be able to preserve the scope.
Maintained Chromium import supports macOS/Linux closed profiles; Firefox import
supports unpartitioned persistent cookies in closed profiles. A session cookie
cannot be imported into Firefox's persistent store without changing its
lifetime, so its adapter rejects it. Safari and Windows Chromium profile import
remain unavailable. `--replace` is required to overwrite an existing cookie.

## Query completion and precedence

```sh
ctx share:browser cookie query --from firefox:personal \
  --site https://example.com --name session --strict --require-match \
  --timeout 30s --to-file ./site-cookies.json
```

Without `--strict`, a query returns available cookies and warnings for sources
or rows it could not read. `--strict` fails on any warning, even when fallback
input provides a missing cookie. `--require-match` fails when no cookies match.
Both failures leave the output file uncreated. Cancellation and overall timeout
are errors; they do not emit a partial CLI result. Library callers can inspect
partial results returned alongside a cancellation error. Cancellation terminates
the adapter invocation and its helper processes, with bounded pipe cleanup. Empty successful
queries return `{"cookies":[]}`.

Merge mode retains the first value for each native cookie scope; it does not
decide which login is freshest. Inline input takes precedence, then ordered
browser sources, then fallback input fills missing scopes. First mode stops at
the first source yielding any match, and uses fallback only if all earlier
sources yielded none. Select explicit profiles when session provenance matters.

## Platform boundaries and validation

Windows Chromium App-Bound `v20` cookies cannot be decrypted by the maintained
standalone adapter. Safari reads its accessible disk cookie store and can miss
normal-session cookies still in memory. An authorized export can supply such
values through inline/fallback input; CTX does not obtain that export itself.
OS credential access and recently uncommitted browser state still affect reads.

Regression fixtures cover source order/trust, filtering, merge/fallback behavior,
partial failures, cancellation, input parsing, and CLI import selection. Native
SQLite fixtures exercise Firefox expiry schemas 15–17, container/partition scope
preservation, and encrypted Chromium import/read/replace with domain binding and
profile locks. Safari fixtures exercise disk reads, stale references, and malformed
records. These checks use synthetic data and do not access a personal
browser profile. Cross compilation confirms build compatibility; real Windows
DPAPI, macOS Keychain/TCC, Linux wallet integrations, and Safari live-session
freshness still require validation on those operating systems and browsers.

Native SQLite operations send SQL through stdin so imported cookie values do
not appear in process arguments. Write failures omit SQL diagnostics that could
repeat a value. Cookie output files are created exclusively with mode 0600;
stdout output requires a pipe.

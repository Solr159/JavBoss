# JAV metadata providers

Dependencies flow from the `jav.MetadataClient` query facade to site packages, then to `metadata`
and shared helpers in `internal/`. Site packages never import the facade or each
other. Database code consumes `metadata` directly; aliases in `jav/types.go` keep
existing metadata consumers compatible.

- `metadata/`: result types, persisted provider IDs, and error identities. Existing
  provider IDs must never be renumbered.
- `provider.go`: small interfaces for individual lookup capabilities. A provider
  implements only supported operations; the facade returns
  `ErrUnsupportedOperation` for missing capabilities.
- `registry.go`, `lookup.go`, `cache.go`: client construction, dispatch, and lookup
  caching. `NewMetadataClient(nil, cache)` builds an independent set of providers. A custom
  registry can be passed for tests. Package-level functions use the default client.
- `avmoo/`, `avsox/`, `javbus/`, `javdatabase/`, `javdb/`, `javdbapi/`, `javmenu/`, `javmodel/`,
  `minnanoav/`, `avwiki/`, `theporndb/`: site implementations and fixture tests. Larger
  implementations separate query flow (`client.go`), transport/session handling
  (`http.go`), and response parsing (`parse.go`).
- `internal/htmlutil/`, `internal/parseutil/`: shared DOM, value, URL, and sample
  image parsing. `internal/avshared/` holds the protocol/template conventions
  shared by Avmoo and Avsox. `internal/ratelimit/` spaces requests.

All network lookups accept a caller context. Provider timeouts derive from that
context, including retries and rate-limit waits. Provider-specific clients,
sessions, and limiters belong to instances. The JavDB HTML and app API providers
live in separate `javdb` and `javdbapi` packages, with independent clients,
request limiters, identities and cache keys. Existing application HTTP/proxy helpers remain in `internal/util`.

When adding a site, implement its supported interfaces, register its constructor in
`newProvider` and its ID in `defaultProviders`, and add fixture/transport tests inside its package. Keep cache
behavior tests at the facade. If parsing changes cached output, update that
provider's cache version without changing existing provider IDs.

Run `GOCACHE=$(pwd)/.gocache go test ./internal/jav/...` from the repository root.

AV Wiki (`avwiki`, provider ID 13) supports actress lookup by Japanese name via
`GET https://av-wiki.net/wp-json/wp/v2/tags`. Queries use `search`, `per_page=100`,
`page`, and `_fields=name,link,description`; the description contains the profile
HTML, so no second page request is needed. The provider follows the API's pagination,
requires an exact name match in both the tag and profile, and parses birth date,
height, measurements, and explicitly listed cup size. Missing fields stay empty.
Romanized names use given-name-first order and initial capitals, with explicitly
capitalized single stage names preserved. The parser matches the family name against
the family-name-first profile slug to handle either source order, keeping the profile's
spelling. Ambiguous names stay empty for other providers to fill. The actress lookup
cache version is bumped when normalization changes; stored idol fields are still only
filled when empty. Alias-only
queries can miss: the tag API does not search aliases inside descriptions.
Requests share a one-second limiter per client and a 15-second lookup deadline.
API errors are not cached as missing actresses. Availability checks perform the
full actress lookup and parse its profile, bypassing lookup caches.

The idol scanner merges profiles in this order: AV Wiki, JavDatabase, JavModel.
MinnanoAV remains available for explicit lookups and availability checks but is
not queried by the background scanner. Existing provider IDs and database rows
remain unchanged.

Availability checks in JAV Providers use these authenticated routes:

- `GET /jav/providers`: list registered providers supporting availability checks
  (`id`, `name`, `domain`, `sample`, and optional `last_result`). The displayed domain comes from each provider's
  availability origin; JavDB's website and API have separate domains.
- `POST /jav/providers/:provider/availability`: check one numeric provider ID using
  the server's saved proxy configuration. No body or query parameters are needed;
  arbitrary target URLs are not accepted. Returns `provider`, `status`,
  `elapsed_ms`, `checked_at` (UTC), and the last `http_status` when an HTTP response
  was received. The previous `/connectivity` route is a compatibility alias that
  performs the same availability check.

The latest completed result per provider is retained in server memory until a
manual check replaces it, proxy settings are saved, or the process restarts. Listing
providers returns these results without making network requests. Manual checks
always make fresh requests. Canceled checks do not replace previous results, and
older concurrent checks cannot overwrite newer checks or repopulate invalidated
results. Nothing is persisted to the database or browser storage.

Network & Proxy stores `proxy_mode` as `auto` (default), `direct` (skip environment
and system proxies), or `manual` (use `proxy_host` and `proxy_port`). Existing
settings without a mode retain manual proxy behavior when a valid port is present.
Mode changes take effect on subsequent requests and clear availability results.
Proxy configuration changes increment an in-memory version. Each HTTP client
checks that version before a request and rebuilds its transport when outdated,
including HTTP/2 connection pools. Identical settings preserve existing pools.
Requests already in progress can finish using their original connection; old
idle connections are closed and busy connections expire after becoming idle.
The last manual address is retained when switching to auto or direct mode.

Each check creates a fresh provider with an independent HTTP client, then calls
the provider directly without creating another `MetadataClient`. It runs one complete
lookup (which may include search, session setup and detail requests). The sample is
`SSIS-001` for movie providers, `030919_047` for Avsox, `波多野結衣` for
JavModel, and `三上悠亜` for MinnanoAV and AV Wiki. A matching movie with a nonempty title or matching actress with profile
fields is required; HTTP 200 alone is never sufficient. Samples are displayed in
the UI, and a missing sample does not imply that all data is unavailable.

Every provider constructor takes an explicit, non-nil `*http.Client`, and provider
requests call `p.httpClient.Do(req)` directly. The registry creates clients with
each site's TLS, timeout and current proxy settings. For normal queries, the
providers using a seven-day URL 404 cache receive a client wrapped with
`util.WithNotFoundCache`; cache policy lives in the registry.

Availability checks create a fresh HTTP client and connection pool before creating
the provider. `util.NewHTTPProbe(httpClient)` adds response-status recording to
that fresh client, which is then injected into the provider. The check never wraps
the client with URL caching. There is no context-based client selection or
request-time cloning. Probe connections are closed when the check finishes.
Scanner sessions, request limiters, device identities and caches are not reused.

`status` is `ok` for a successful data lookup, `not_found` when the sample is absent,
`invalid_response` for invalid/incomplete data, `http_error` for an unsuccessful
lookup whose last response is 4xx/5xx, or `timeout`, `canceled`, `dns_error`,
`tls_error`, `network_error`. No raw errors, credentials, or retrieved content are
returned to the UI. Checks have a 30-second upper deadline (shorter provider/client
timeouts still apply). The frontend runs at most three checks concurrently and
cancels pending work when the panel closes or proxy settings change. New providers
should implement `ProviderOrigin` and a supported movie or actress-name lookup.

Failed checks log the provider, failure status, last HTTP status, elapsed time and
error cause. Request URL wrappers are removed from logged errors to omit URL
credentials and query parameters. Canceled checks use informational logging.

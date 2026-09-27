# JAV metadata providers

Dependencies flow from the `jav` query facade to site packages, then to `metadata`
and shared helpers in `internal/`. Site packages never import the facade or each
other. Database code consumes `metadata` directly; aliases in `jav/types.go` keep
existing metadata consumers compatible.

- `metadata/`: result types, persisted provider IDs, and error identities. Existing
  provider IDs must never be renumbered.
- `provider.go`: small interfaces for individual lookup capabilities. A provider
  implements only supported operations; the facade returns
  `ErrUnsupportedOperation` for missing capabilities.
- `registry.go`, `lookup.go`, `cache.go`: client construction, dispatch, and lookup
  caching. `NewClient(nil, cache)` builds an independent set of providers. A custom
  registry can be passed for tests. Package-level functions use the default client.
- `avmoo/`, `avsox/`, `javbus/`, `javdatabase/`, `javdb/`, `javmenu/`, `javmodel/`,
  `minnanoav/`, `theporndb/`: site implementations and fixture tests. Larger
  implementations separate query flow (`client.go`), transport/session handling
  (`http.go`), and response parsing (`parse.go`).
- `internal/htmlutil/`, `internal/parseutil/`: shared DOM, value, URL, and sample
  image parsing. `internal/avshared/` holds the protocol/template conventions
  shared by Avmoo and Avsox. `internal/ratelimit/` spaces requests.

All network lookups accept a caller context. Provider timeouts derive from that
context, including retries and rate-limit waits. Provider-specific clients,
sessions, and limiters belong to instances. The JavDB HTML and app API providers
have distinct identities and cache keys; `javdb.NewProviders` gives them a shared
site limiter. Existing application HTTP/proxy helpers remain in `internal/util`.

When adding a site, implement its supported interfaces, register it in
`defaultProviders`, and add fixture/transport tests inside its package. Keep cache
behavior tests at the facade. If parsing changes cached output, update that
provider's cache version without changing existing provider IDs.

Run `GOCACHE=$(pwd)/.gocache go test ./internal/jav/...` from the repository root.

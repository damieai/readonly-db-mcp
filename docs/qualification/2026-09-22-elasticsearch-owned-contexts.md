# Elasticsearch owned PIT/scroll qualification — 2026-09-22

Status: implementation and TLS/MCP fixture evidence. This is **not** real-engine
certification of Elasticsearch 8.19.21 or 9.1.10. SQL cursors and language profiles
are outside this completed implementation increment.

## Implemented behavior

- `es_cursor` opens a PIT or scroll and returns its first page, advances an owned
  handle, or closes it. Handles bind a cryptographic nonce to the target instance,
  actual MCP transport session, authority revision and original scope. The stdio
  SDK's empty session ID is never used as an ownership key.
- Native ID rotation stays internal. PIT continues using its original resolved
  indices and mapping proof across alias rollover. Each page proves live embedded
  lookup sources, including runtime lookup mappings. No operation restarts an
  expired/failed snapshot. Raw IDs, foreign handles and mass clear are rejected.
- Native scripts, supplied-vector queries, sorting, slices and aggregations use
  the existing contextual visitors. PIT next without a body uses retained DSL
  and the last hit's exact `search_after`; an explicit body supports composite
  continuation and other native query changes. Scroll keeps its original body
  and page size. Empty ordinary hit pages close; aggregation-only PITs remain
  available for explicit continuation/close.
- `es_batch.consistency=pit` preflights every member, checks compatible source and
  routing options, opens one PIT, uses the newest returned ID for each search,
  then closes it. Non-search members and incompatible scopes fail before creation.
  Each member and the batch report PIT consistency. Independent batches retain
  their existing semantics.
- Expiry, session disconnect, observed permission-proof changes, stale authority
  and shutdown trigger cleanup. Changed DLS/FLS values change the authority hash.
  Advances and closes serialize per entry; cleanup does not require a healthy
  authority proof. Unknown outcomes retain capacity and are never replayed.
- The mixed `es_cursor` tool is read-only and non-idempotent. Persistent document,
  index, security and administrative mutations remain denied.

## Resource contract

| Resource | Current implementation |
| --- | --- |
| Per-target contexts | Default 32; configurable 1–128 |
| Process contexts | Fixed 128, reserved before native creation |
| Retained context memory | Separate fixed 64 MiB process budget; included in forecast |
| Idle lease | Default 5m; configured 1s–15m; tool override 1ms–15m |
| Absolute handle lifetime | Default 30m; at least configured idle, maximum 2h |
| Native ID / retained sort bytes | Default 64 KiB; configurable 1 KiB–1 MiB |
| Recovery | Fixed 5s maintenance attempt; no retry of creation/advancement |
| Cleanup response | At most 64 KiB, additionally bounded by target result limit |

Idle handles retain neither an execution permit nor an active connection.
Normal cleanup shares the request deadline. Cancellation disables the handle,
releases request admission and schedules bounded maintenance recovery. Cleanup
also reserves response memory. Lease timers release local reservations even if
shutdown cannot obtain cleanup resources. Renewal is rechecked under the entry
gate before a timer releases a reservation.

Unconfirmed native creation/rotation keeps its slot and retained-memory charge
through the greatest requested native keep-alive plus the maximum request
interval. Shorter renewals do not reduce this native retention allowance; local
idle/absolute deadlines still apply. This is conservative client accounting,
not independent proof of server task termination. Measuring the native lifetime
and cancellation bound, including a disconnected coordinator, remains a release
gate. A successful clear of a known latest ID releases its reservation; clearing
a predecessor after an unknown rotation does not prove the unknown context ended.

## Evidence

`internal/dialects/elasticsearch/cursor_test.go` covers both pinned profiles for
rotating IDs and cleanup,
native number precision, foreign/raw IDs, scope replacement, advanced vector DSL,
PIT alias rollover, live runtime lookups, composite continuation, exhaustion,
partial results, malformed creation, failed cleanup, quota retention, shorter
renewals, permission/DLS/FLS revision, idle/absolute expiry, session cleanup, shutdown,
compatible PIT batches, preflight rejection, concurrent advance/close and
cancellation that returns before blocked recovery.

`internal/mcpserver/elasticsearch_test.go` exercises the registry and raw MCP
tools over two separate in-memory transport sessions. It checks non-idempotent
cursor annotations, ownership isolation despite empty SDK session IDs, exact
large integers, malformed/forged envelopes and cleanup after disconnect.
Configuration tests cover defaults, ceilings and lifetime consistency.

Validation commands:

```sh
go test -race ./...
go vet ./...
go build -trimpath -o /tmp/readonly-db-mcp-es-contexts ./cmd/readonly-db-mcp
python3 tools/es-rest-catalog/generate.py --cache /tmp/readonly-es-rest-specs --check
git diff --check
```

## Remaining live gates

The existing opt-in `TestElasticsearchLiveReadOnlyMetadata` now includes PIT,
scroll continuation/close and PIT batches in addition to metadata/query and the
native write-denial probe. Configure `READONLY_DB_MCP_ES_CONFIG`,
`READONLY_DB_MCP_ES_TARGET` and `READONLY_DB_MCP_ES_WRITE_PROBE_INDEX` as described
in the earlier qualification record. No ES endpoint was provisioned for this
increment, so the live test is skipped in this environment.

Before deployment qualify both pinned builds, actual privilege actions and
licenses, PIT snapshot behavior under updates/rollover, sliced scans, rotated IDs
and context counts after disconnect/timeouts, DLS/FLS revision behavior,
server-side cancellation, proxy behavior and RSS/socket saturation. Query-language
profiles, API-key/attestor, custom plugins/inference, cross-cluster and dynamic
suggestion collate remain separately reported pending capabilities.

Protocol references: [PIT creation and continuation](https://www.elastic.co/docs/api/doc/elasticsearch/v8/operation/operation-open-point-in-time),
[scroll continuation](https://www.elastic.co/docs/api/doc/elasticsearch/operation/operation-scroll),
and [PIT cleanup](https://www.elastic.co/docs/api/doc/elasticsearch/operation/operation-close-point-in-time).

The retention high-water mark was also checked against the pinned
[ReaderContext implementation](https://github.com/elastic/elasticsearch/blob/4fe44c255c3d0da06779b921e132cdc555ed9aff/server/src/main/java/org/elasticsearch/search/internal/ReaderContext.java),
which accumulates keep-alive with a maximum.

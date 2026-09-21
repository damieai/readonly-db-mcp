# Elasticsearch phase 1 implementation and qualification

This phase implements the first rollout step of RFC-0006 and a usable scoped
metadata path. It does not complete Elasticsearch query support. The prior
Redis/SQL Server acceptance record remains separate; this work does not claim
to have run their pending real-server matrices.

## Available behavior

- `engine: elasticsearch`, independent registry/core capability and `es_metadata`.
- `resolve` returns native index, alias and data-stream resolution. `mappings`
  resolves first, preserves the requested alias/stream expression, and checks
  returned concrete names before exposing native mapping JSON.
- `inspect_target` reports proof freshness, index scope, the pinned version and
  per-capability implementation status. It never claims a server read-only mode.
- Dedicated username/password credentials use the existing file/env secret
  loader. Every configured origin must authenticate the same enabled realm user,
  match the cluster UUID/build and expose complete effective self privileges.
- Allowed authority is cluster `monitor`/`none` and scoped index
  `read`/`view_index_metadata`/`none`. Restricted, run-as, application, global,
  remote, write and unknown privileges fail proof. Native DLS/FLS restrictions
  are preserved; the adapter does not reinterpret them as grants.
- Glob automata prove containment/disjointness for future index names. Current
  alias/data-stream expansions are checked separately. Hidden `.ds-*` backing
  indices are accepted only through an allowed resolved stream, with explicit
  denies still enforced. A mixed allowed/denied expansion fails.
- Effective authority is refreshed on a maintenance permit at half the configured
  proof interval. Expired or failed proof blocks new calls; 401/403 invalidates it.

## Transport and data boundaries

The official `elastic-transport-go/v8` dependency is pinned to **v8.11.0**.
REST profiles come from these immutable source commits:

| Version | Commit/build pin | Qualification |
| --- | --- | --- |
| 8.19.21 | `4fe44c255c3d0da06779b921e132cdc555ed9aff` | Local TLS fixtures; real engine pending |
| 9.1.10 | `f2e019bd8110088070638ca779ec1543188c0f43` | Local TLS fixtures; real engine pending |

The generated catalog records each REST source URL and SHA-256. Effects are
reviewed independently: POST search is a read, owned PIT deletion is lifecycle
cleanup, and GET-capable refresh/flush endpoints are administrative mutations.
Only resolve/mappings have public handlers in this phase. A build pin is an
operator identity check, not cryptographic attestation of a remote executable.

HTTPS verifies each configured hostname against the supplied CA. Environment
proxies, redirects, automatic node discovery and SDK retries are disabled.
HTTP/1.1 sockets share one target-wide lease budget across all endpoints; idle
counts share one transport pool. Expired idle connections close; active work
finishes before lifetime retirement. Request context cancellation reaches the
HTTP exchange. Go's standard transport can still recover a stale reused
connection before a safe metadata read; no stateful operation is enabled here.

Native JSON uses raw bytes. Strict validation rejects duplicate keys, invalid
UTF-8, extra JSON values and excessive depth/nodes. The MCP tool uses a raw
handler because the SDK's generic schema/default processing normalizes through
floating-point values and discards duplicate keys before custom unmarshaling.
Both structured and text result copies are budgeted, including an envelope
allowance; oversized results fail rather than truncating metadata.

Requests inherit the five-minute default, ten-minute normal maximum and
fifteen-minute hard ceiling. A process-wide 64 MiB queue-byte budget and 512 MiB
active ES adapter budget supplement shared admission. Active reservations are
`6 * max_result_bytes + 64 * max_json_nodes + max_request_bytes + 2 MiB`.
The glob proof also caps retained state, state count and transitions. These
coefficients are conservative reservations, not measured RSS guarantees.
Delivery backlog, allocation multipliers and full-process saturation remain
qualification work; no live ES server memory/cancellation claim is made.

## Evidence and reproduction

Tests exercise real TLS sockets with an in-process HTTP fixture, the official
Elastic transport, production config/registry wiring and an in-memory MCP
client/server session. The fixtures do not emulate Elasticsearch execution or
prove its native permission implementation.

Covered assertions include:

- Every configured endpoint is checked; wrong cluster/build, disabled security,
  write/unknown privileges and stale/drifting proofs fail closed.
- Future wildcard escapes, alias redirection and hidden stream backing checks.
- Unknown envelope fields, path/query/header injection and duplicate JSON keys.
- No redirect/status retry, sanitized error bodies and permission invalidation.
- Raw large-integer preservation, decoded-byte/JSON/envelope limits and rejection
  rather than partial mapping truncation.
- Shared sockets across origins, concurrent wire requests independent of public
  admission, cancellation/permit/memory recovery, idle retirement and shutdown.
- Separation from SQL/Redis targets, MCP tool schema/annotations and raw binding.

Validation commands:

```sh
go test -race ./...
go vet ./...
go build -trimpath -o bin/readonly-db-mcp ./cmd/readonly-db-mcp
python3 tools/es-rest-catalog/generate.py --cache /tmp/readonly-es-rest-specs --check
git diff --check
```

The repository race suite, vet, binary build, catalog reproduction and diff
whitespace check all passed during this phase.
The live ES test skips without an explicit disposable configuration; that skip
does not count as engine qualification.

## Provisioning and live gate

Use [the dedicated example](../../configs/elasticsearch.example.yaml). Replace
its UUID and endpoint with the actual pinned build/cluster, and supply the CA
and realm-user password through protected local configuration. No query tool
accepts hosts, credentials, headers or HTTP paths.

Provision a role with cluster `monitor`, plus `read` and `view_index_metadata`
only on the desired index expressions, and `allow_restricted_indices: false`.
Do not assign write, security-management, run-as or broad inherited roles. If a
configured denied pattern overlaps the account's grants, narrow the native grants
as well; a currently empty denied index is not proof about future indices.

Example tool calls:

```json
{"target":"search-reporting","operation":"resolve","indices":["reports-*"]}
```

```json
{"target":"search-reporting","operation":"mappings","indices":["reports-2026"]}
```

For real-engine qualification, create a disposable `mcp-es-acceptance-*` index
and a dedicated corresponding read-only role/user. Use `environment: test` and
that exact scope in a separate local config. The test bypasses adapter policy
for one randomized document-write probe and requires native HTTP 403; never run
it against a business-data target. Administrator credentials are not supplied
to the test process.

```sh
export READONLY_DB_MCP_ES_CONFIG=/absolute/path/to/es-acceptance.yaml
export READONLY_DB_MCP_ES_TARGET=es-acceptance
export READONLY_DB_MCP_ES_WRITE_PROBE_INDEX=mcp-es-acceptance-fixture
go test ./internal/dialects/elasticsearch -run TestElasticsearchLiveReadOnlyMetadata -count=1 -v
```

## Next stages and open gates

1. Native DSL/document reads, complete embedded-source visitors, scripts/runtime
   fields, aggregation, templates and vector/hybrid positives. No tiny-query or
   keyword-based substitute satisfies this stage.
2. Owned PIT/scroll/cursors, batch consistency, SQL/ES|QL/EQL parser helpers,
   date-math sources, inference/plugin and explicit cross-cluster proof profiles.
3. API-key descriptor intersection/expiration and optional read-only attestor;
   complete licensed/version capability matrices.
4. Real native write-denial, alias/rollover races, long-running server-side task
   cancellation, mixed-load fairness, delivery backlog/RSS and proxy profiles.

The relevant security semantics are documented by Elastic in
[effective self privileges](https://www.elastic.co/docs/api/doc/elasticsearch/operation/operation-security-get-user-privileges),
[index resolution](https://www.elastic.co/guide/en/elasticsearch/reference/8.19/indices-resolve-index-api.html)
and [transport configuration](https://www.elastic.co/docs/reference/elasticsearch/clients/go/configuration).

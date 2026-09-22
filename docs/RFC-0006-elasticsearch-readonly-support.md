# RFC-0006: Elasticsearch Read-Only Query Support

- Status: Metadata, native query/batch, owned PIT/scroll/SQL and synchronous EQL/SQL/ES|QL implemented; ENRICH isolation, other advanced profiles and live qualification pending
- Authors: readonly-db-mcp maintainers
- Created: 2026-09-12
- Depends on: RFC-0001 resource governance and the common architecture contract
- Target release: staged; advanced query acceptance is a completion gate
- Scope: Elasticsearch over HTTPS through the existing stdio MCP transport

Implementation update (2026-09-21): the first rollout stage now includes
configuration/registry wiring, fixed metadata tools, the official Go transport,
pinned 8.19.21/9.1.10 build identities, dedicated-user privilege proofs, glob
containment, scoped index/alias/data-stream resolution, native mappings and TLS
adversarial fixtures. `es_metadata` is available; `es_query`, `es_batch`,
`es_cursor`, API-key/attestor, language and cross-cluster profiles remain pending.
The selected builds are compatibility pins, not certified deployments. See
[phase 1 evidence](qualification/2026-09-21-elasticsearch-phase-1.md).

Implementation update (2026-09-22): `es_query` and independent `es_batch` now
execute native search/count, document/term-vector reads, diagnostics and templates.
Contextual visitors cover compound queries, embedded sources, aggregations,
Painless/expression scripts, runtime lookups, supplied vectors and local fusion.
Compatible batches use service-built `_msearch`; all members share preflight,
deadline and output limits. Stored definitions are inspected and frozen; rendered
templates are checked before execution. Stock node/plugin inventories are proved
for queries. This is a partial implementation of the full RFC, with fixture
coverage rather than real-engine certification. See the
[native query evidence and remaining gates](qualification/2026-09-22-elasticsearch-native-queries.md).

Implementation update (2026-09-22, owned contexts): `es_cursor` now opens,
advances and closes session-owned PIT/scroll handles, including rotating native
IDs, explicit PIT aggregation continuation, idle/absolute expiry, authority
revision invalidation and bounded cleanup. `es_batch.consistency=pit` shares
one PIT across compatible preflighted searches. Context reservations survive
ambiguous outcomes; canceled requests schedule maintenance recovery. The current
profile fixes process limits at 128 contexts / 64 MiB retained state and recovery
at 5 seconds; the broader configurable ceilings below remain proposals. SQL
cursors and live-server lifecycle/cancellation qualification remain pending.
See [owned-context evidence](qualification/2026-09-22-elasticsearch-owned-contexts.md).

Implementation update (2026-09-22, EQL): `es_query.operation=eql.search` and
independent EQL batch members now use a Go parser generated from the complete,
identical EQL grammars at both pinned commits. Original language text, advanced
expressions and native sequence/sample result structures are preserved. The
adapter separately proves index scope, filter references and runtime lookups,
enforces synchronous execution/completeness, and budgets parser state without
global per-query DFA retention. Java is generation-only. Native semantic
availability remains an ES decision; this does not enable SQL, ES|QL, SQL cursors
or persisted async language results. Live-engine certification remains pending.
See [EQL qualification and parser packaging](qualification/2026-09-22-elasticsearch-eql.md).

Implementation update (2026-09-22, SQL): native `sql.query`, `sql.translate`
and `es_cursor.kind=sql` now use separately generated complete SQL grammars from
both pinned commits. Query text/parameters are preserved; relation, metadata,
catalog, filter/runtime and full-text inference source/effect checks precede
execution. Stateless queries drain bounded native pages; explicit SQL cursors
retain row/columnar output and use owned fetch/clear lifecycle, including terminal
nonempty pages, session cleanup and uncertain-outcome reservations. SQL/EQL share
one parser memory pool. ES|QL and persisted async profiles remain pending.
See [SQL evidence and remaining live gates](qualification/2026-09-22-elasticsearch-sql.md).

Implementation update (2026-09-22, ES|QL): `esql.query` and independent batch
members now execute original native pipelines using complete separately generated
grammars/imports for both pins. Structured visitors cover FROM, LOOKUP JOIN,
quoted source lists, 8.19 EXPLAIN and 9.1 FORK, native parameters and full-text
field lineage. JSON row/columnar results, profiling and number precision are
preserved; incomplete, oversized and unexpected asynchronous results fail whole.
All three languages share the existing parser pool. ES|QL reserves an 8 MiB
baseline, informed by its larger parser's allocation samples. ENRICH is parsed
but remains unavailable until historical enrichment data and future policy
changes have an explicit isolation/authority proof; source-index DLS/FLS alone
cannot provide that proof. Inference, experimental pragmas, persisted async and
live certification remain separate gates. See
[ES|QL evidence and remaining gates](qualification/2026-09-22-elasticsearch-esql.md).

## Summary

Add `engine: elasticsearch`, an independent Elasticsearch adapter, and native
MCP query tools. Preserve Elasticsearch's expressive query surface: Query DSL,
aggregations, search scripts, runtime fields, templates, vector/hybrid search,
deep pagination, SQL, ES|QL and EQL. A simple search implementation alone does
not complete this RFC.

The policy is based on effects and authorization. Forbid persistent changes to
documents, indices, mappings, configuration, security and other durable objects.
Do not reject operations merely because they use POST, scripts, joins, regular
expressions, complex aggregations or expensive query features. Resource limits
are explicit availability controls, independently configurable from mutation
policy. The final write boundary is an attested Elasticsearch identity.

The full query design below remains the completion contract. Only the delivered
subsets described above are enabled in the current binary; unimplemented reads
are reported as unavailable capabilities, not mutations.

## Context and problem

Repository inspection on 2026-09-12 finds four engine constants and registry
constructors: MySQL, PostgreSQL, SQL Server and Redis. The corresponding
`internal/dialects` packages exist. There is no Elasticsearch configuration,
driver, registry entry, core capability or MCP tool, and no OpenSearch adapter.

| Engine | Repository status | Outstanding qualification |
| --- | --- | --- |
| MySQL | Adapter and tests exist | Validate provisioned identities and deployed builds |
| PostgreSQL | Adapter and tests exist; RFC-0002's status header predates implementation | Live deployment qualification |
| Redis | Standalone, Sentinel, Cluster and attested module infrastructure exist | Module index-prefix introspection; real topology/failover and module certification gates |
| SQL Server | Runnable implementation; RFC-0005 partially implemented | ScriptDom hardening, transitive module proof, optional attestor and exact-version matrices |
| Elasticsearch | No adapter; proposed here | Entire implementation and acceptance matrix |
| OpenSearch, MongoDB, SQLite, Oracle, ClickHouse, Cassandra, Neo4j, other engines | No independent adapters | Not scheduled by this RFC |

This is an inventory, not a claim that every missing engine is on an agreed
roadmap. Protocol-compatible products also need qualification before being
advertised as supported.

Elasticsearch requires a separate design because it has an HTTP operation
surface, nested JSON results, aliases/data streams, index references embedded
inside query bodies, and server-side search contexts. The SQL transaction
interface and SQL row normalization cannot express all of these correctly.

## Goals

- Expose native reads with full semantic results, including aggregation trees,
  hit metadata, vector scores, EQL sequences and tabular language results.
- Cover advanced native queries in positive integration tests, together with
  nearby mutation and scope-escape counterexamples.
- Enforce target, index, alias, data stream and indirect-source authorization.
- Support dedicated username/password and API-key credentials with independently
  verified effective authority; never infer safety from a successful search.
- Preserve the five-minute default and ten-minute configurable request budget.
- Support bounded PIT/scroll/cursor lifecycles without holding connections or
  execution permits while an agent reasons between tool calls.
- Report precisely whether a feature is available, version/license dependent,
  awaiting implementation, unauthorized, or outside resource limits.

## Non-goals

- Document ingestion or modification, reindexing, schema or security changes,
  index management, lifecycle management, transforms, jobs or cluster control.
- A generic HTTP proxy with caller-selected methods, paths, hosts or headers.
- Making the whole Elasticsearch process physically read-only: normal search
  execution may use caches, logs, temporary memory and Lucene search contexts.
- Promising that Elasticsearch has SQL transactions or that independent
  searches share a snapshot.
- Treating OpenSearch as interchangeable with Elasticsearch, or claiming
  Serverless/legacy releases are certified without their own test profiles.
- Disabling advanced queries to make policy implementation easier.

## Proposed design

### Adapter and version profiles

Add `internal/dialects/elasticsearch` with transport, operation catalog, scope
resolver, permission attestation, language analysis, execution, context manager
and result-budget components. Add `core.ElasticsearchTarget`, native request
and response types, and `Registry.GetElasticsearch`. Register ES tools beside
SQL and Redis tools; do not funnel ES queries through `query_select`.

Use the official Go client transport behind an internal interface, preserving
JSON via `json.RawMessage`/`json.Number`. Select and pin exact client and server
versions during implementation. Certify an 8.19.x profile and a selected 9.x
profile separately; these are candidate test lines, not blanket support claims.
Each profile records build identity, REST specification revision, script/plugin
inventory, language grammar, privilege semantics, license-dependent capabilities
and cancellation behavior. Unknown profiles fail startup with an actionable
compatibility error. Mixed-version upgrades require an explicitly tested pair.

Generate the operation catalog from pinned upstream REST specifications, then
review effects, source locations and lifecycle behavior per operation. The
catalog is a security boundary for endpoint effects, not a tiny DSL whitelist.
For certified built-in query families, preserve every documented field; reject
unknown executable/source-bearing extensions until their profile is reviewed.
Ordinary field names, literal data and aggregation names are never compared to
mutation-keyword lists.

### Public contracts

| Tool | Proposed contract |
| --- | --- |
| `es_query` | `{target, operation, indices?, body?, options?, timeout_ms?, max_rows?, purpose?}`; native JSON query body |
| `es_batch` | `{target, requests, consistency, timeout_ms?}`; typed operations, one shared deadline |
| `es_metadata` | `{target, operation, indices?, fields?, fresh?}`; scoped resolve, mappings, aliases, data streams and field capabilities |
| `es_cursor` | `{target, action, handle?, indices?, body?, keep_alive_ms?, timeout_ms?}`; open/next/close a service-owned context |

`operation` is a semantic enum, not an HTTP path. `options` is validated against
the versioned endpoint schema; routing, preference, search type, tracking,
source filtering and typed aggregation keys remain available. Reject unknown
envelope fields, duplicate JSON keys, multiple JSON values and invalid UTF-8
before constructing a request. All indices/IDs are structured values encoded
once into a fixed route. No raw NDJSON, URL fragments or query-string injection.
`es_batch` constructs `_msearch`/`_mget` framing itself after validating every
member, including member-specific index declarations.

Return `{query_id, target, engine, consistency, operation, data, duration_ms,
partial, truncated, warnings, continuation?}`. `data` preserves the native JSON
shape and numeric precision. Keep ES `took`, shard outcomes, `timed_out`, hit
total relation, approximation metadata, sort values and composite `after_key`.
Represent missing values distinctly from nulls. Failures use stable codes such
as `mutation_forbidden`, `scope_denied`, `capability_unavailable`,
`resource_limit`, `deadline_exceeded`, `permission_proof_stale` and
`cursor_expired`; never mislabel an unimplemented feature as a mutation.

`inspect_target` exposes the certified profile, non-secret capabilities,
allowed index expressions, consistency and proof freshness. Elasticsearch has
no general read-only transaction flag: do not set `ServerReadOnly=true` merely
because the principal is attested. Lifecycle tools get operation-appropriate
MCP idempotency annotations; advancing a scroll is not idempotent.

### Required operation and advanced-feature coverage

| Family | Included behavior | Effect/scope boundary |
| --- | --- | --- |
| Document reads | Get, multi-get, source, exists, term vectors/multi-term vectors | Validate every document's index; artificial documents stay request-local |
| Query DSL | Bool, full-text, query string, intervals/span, fuzzy/regexp/wildcard, nested, parent/child, geo, percolate, rescore, collapse, highlighting, suggest | Resolve embedded document/index references; no complexity-based denial |
| Aggregation | Bucket/metric, nested/reverse-nested, pipeline, composite pagination, scripted metric | Preserve exact aggregation semantics; bound output and execution separately |
| Scripts/runtime fields | Inline and stored scripts in query, score, sort, field and aggregation contexts; runtime mappings and lookup fields | Certify language/context; resolve lookup targets; script catalog writes forbidden |
| Vector and hybrid | Supplied-vector kNN, script scoring, retrievers, fusion/reranking where supported | Local computation and approved inference have separate capabilities |
| Search diagnostics | Count, validate-query, document explain, search `profile`, search shards and field capabilities | Profile executes a search and consumes the ordinary execution budget |
| Templates | Inline/stored Mustache templates, including multi-search templates | Render, parse and validate resulting DSL before executing those exact rendered bytes |
| Deep pagination | `search_after`, composite `after_key`, PIT, scroll/sliced scroll and SQL cursor | Service-owned handles; preserve sort and snapshot semantics |
| SQL | Native read-only SQL, native parameters, translate and cursor fetch/clear | Engine-specific parser and source resolution; no MySQL grammar reuse |
| ES\|QL | Native pipelines, aggregation, expressions, LOOKUP JOIN, query-time ENRICH and supported read-only commands | Resolve every source including joins, views, policies and implicit lookups |
| EQL | Event queries, sequences and supported joins | Resolve indices; preserve sequence structure and distinguish incomplete results |
| Cross-cluster reads | Configured remote aliases and permitted remote index expressions | Both local and remote privilege/scope proof required; no caller-created remote destinations |

The ordinary Search API already exposes substantial query and response
structure; the adapter must preserve it rather than translating it to a small
search form. See the [Search API contract](https://www.elastic.co/docs/api/doc/elasticsearch/operation/operation-search).

Painless assignments, loops, `emit`, and mutations to request-local
`scripted_metric` state are valid calculations, not document writes. Native
Painless, expression and Mustache contexts are certified using their sandbox
and execution contract. A custom script engine/plugin needs an effect and
network capability profile; the word `script` is never sufficient to reject a
query. See [scripting security](https://www.elastic.co/docs/explore-analyze/scripting/modules-scripting-security)
and [script languages](https://www.elastic.co/docs/explore-analyze/scripting).

Stored templates are rendered once, and the resulting JSON is validated and
sent as a normal search. Do not validate a template and then execute its mutable
ID again. Stored executable scripts need current language/context attestation;
if they are materialized inline to freeze a definition, the profile must prove
equivalent behavior. See [render search template](https://www.elastic.co/guide/en/elasticsearch/reference/current/render-search-template-api.html).

SQL/ES|QL/EQL use maintained upstream grammars or a version-pinned helper with
a structured AST/source contract. The implementation must decide and test the
parser/helper packaging before enabling those operations; keyword scanning is
insufficient. Language literals and comments cannot synthesize new sources.
Parse all source commands, subqueries, join branches and view expansions.
The native server still parses/executes the original admitted query with native
parameters; do not silently rewrite a complex query into a weaker dialect.

ES|QL LOOKUP JOIN reads a lookup index. ENRICH consumes an existing enrichment
policy at query time; creating or executing the policy is a separate prohibited
write. ENRICH's documented lack of policy-level/DLS/FLS isolation requires an
explicit proof that all reachable enrichment data is in this target's scope;
use a separately scoped target/cluster when that cannot be established. Do not
claim ordinary source-index grants alone secure enrichment.
See [LOOKUP JOIN](https://www.elastic.co/docs/reference/query-languages/esql/commands/lookup-join),
[ENRICH security prerequisites](https://www.elastic.co/docs/reference/query-languages/esql/esql-enrich-data)
and [ES|QL REST authorization](https://www.elastic.co/docs/reference/query-languages/esql/esql-rest).

Text-to-vector builders, semantic search, rerankers and inference can reach
operator-configured external services. Preserve supplied-vector/local scoring
without an inference dependency. Offer inference only with an explicit profile
binding endpoint/model identities, egress destinations, credentials, data scope
and cost limits; never let a request configure an inference endpoint or start a
deployment. An unavailable inference proof affects that capability, not kNN or
all search. Same principle applies to certified custom plugins.

### Effect classification and temporary state

Reject index/create/update/delete/bulk, update/delete-by-query, reindex,
mapping/settings/alias/data-stream changes, refresh/flush/force-merge controls,
snapshot/restore, security management, stored script/template writes, enrich
policy execution, transforms, task-wide cancellation and configuration APIs.
Classify the actual operation: POST search is a read; DELETE of a service-owned
PIT is cleanup. Neither HTTP methods nor index `read` privileges alone define
the public operation boundary.

PIT, scroll and synchronous SQL cursors are explicitly allowed bounded search
state. Creating/renewing/closing them may retain segments or allocate memory,
but does not authorize a durable business object change. Return unguessable
local handles bound to target, process/session, credential revision, scope,
context kind and expiry. Never accept raw server IDs from callers, including
IDs smuggled through native bodies. Internally translate bound handles to IDs.
Reject `_all` cleanup and cross-session handles; track rotated IDs, serialize
advance/close, and clear contexts on close, expiry, shutdown or proof drift.
See [PIT semantics and resource costs](https://www.elastic.co/docs/api/doc/elasticsearch/v8/operation/operation-open-point-in-time).

Async search and async SQL/ES|QL/EQL need separate lifecycle profiles. Some
async APIs persist results even though their purpose is reading. Do not silently
enable them under a no-persistent-modification guarantee or equate
`keep_on_completion=false` with no stored state. The initial strict profile
uses synchronous execution and disables async result retention. A profile may
enable a demonstrably non-persistent mode after exact-version tests; otherwise
persisted async results require an explicit amendment to the security objective.
This is an effect-based exclusion, not an exclusion of long or advanced queries.
See [async search storage](https://www.elastic.co/docs/api/doc/elasticsearch/operation/operation-async-search-submit)
and [async SQL retention](https://www.elastic.co/docs/reference/query-languages/sql/sql-async).

### Scope resolution and consistency

Validate explicit paths AND embedded sources: terms lookup, indexed geo-shapes,
percolator documents, more-like-this documents, runtime lookup fields,
multi-request headers, SQL catalogs, ES|QL joins/ENRICH/views and remote aliases.
Generate these source visitors from the certified schemas where possible.

Allow configured index patterns, date math, aliases and data streams. Resolve
them under the execution identity, then check allowed/denied policy and server
grants. An omitted index list means configured scope, never unrestricted
`/_search`. A mixed allowed/denied expansion fails rather than silently changing
the query to a smaller data set. Restricted/system indices are excluded except
for explicitly attested internal read behavior such as ENRICH; hidden backing
indices of allowed data streams are not categorically denied.

Preserve alias filters and routing: do not replace a filtered alias with its
backing index in the execution request. Alias filters are not confidentiality
controls for all APIs (notably direct document reads); require native DLS/FLS
or separately scoped indices when confidentiality is intended. The server's
scoped grants are the final race boundary for alias rollover, index creation
and re-pointing. Where aliases can grant more than the physical-index policy,
require grants that enforce that policy or reject the target configuration.

Wildcard grant containment must account for future matching names, not only
today's expansion. Prove pattern inclusion under Elasticsearch's pattern
semantics; reject a configuration that cannot prove it. Revalidate credentials
and effective authority periodically (default 1 minute, maximum 5 minutes),
and on authentication/authorization changes. Stale proof marks the target
unhealthy, closes handles, and blocks subsequent dispatch. Trusted administrator
changes can occur between checks; periodic inspection is not an atomic lock on
security state. Strict server grants and controlled provisioning remain required.

Default search consistency is `eventual`. Get/multi-get may be real-time and
must report that distinction. `es_batch.consistency=independent` runs independent
reads; `pit` opens one PIT over the permitted scope and uses it for compatible
search members. Reject incompatible operations in a PIT batch. Never label
multi-get, language cursors or ordinary `_msearch` as a common transaction.
Later PIT pages use the same snapshot; shard failures/expiry do not silently
restart the query against fresh data.

### Proposed configuration

Illustrative full-stage configuration. Phase 1 uses `version` and `cluster_uuid`
with username/password instead of the proposed API-key/profile fields below;
see `configs/elasticsearch.example.yaml` for its runnable configuration shape:

```yaml
targets:
  search_reporting:
    engine: elasticsearch
    environment: production
    consistency: eventual
    elasticsearch:
      endpoints: [https://es-reporting.example.invalid:9200]
      profile: es-8.19-certified-build
      api_key_file: /run/secrets/es-reporting-api-key
      allowed_indices: [reports-*, events-*]
      denied_indices: [events-private-*]
      privilege_recheck_interval: 1m
      remote_clusters: []
      max_request_bytes: 1048576
      max_json_depth: 128
      max_open_contexts: 32
      context_keep_alive: 5m
      max_context_lifetime: 30m
    tls:
      mode: verify-full
      ca_file: /run/secrets/es-ca.pem
    connection:
      max_open: 8
      max_idle: 4
      connect_timeout: 15s
      write_timeout: 15s
      read_timeout: 11m
      max_lifetime: 30m
      max_idle_time: 10m
```

Allow exactly one secret source: API key file/env, or dedicated username plus
password file/env. Reuse existing secret-file checks and TLS policy. ES config
does not require a fake SQL database/schema/port; reject conflicting SQL/Redis
blocks. `remote_clusters` is an operator-owned alias-to-scope/profile map and
does not create or update Elasticsearch remote-cluster settings.

### Example advanced read

This proposed `es_query` call combines a runtime script with composite metric
aggregation. The script computes a result without updating a document. The
caller can paginate with the returned composite `after_key`; adapter-injected
early termination must not change the sums.

```json
{
  "target": "search_reporting",
  "operation": "search",
  "indices": ["reports-*"],
  "body": {
    "size": 0,
    "runtime_mappings": {
      "amount_with_tax": {
        "type": "double",
        "script": {
          "source": "emit(doc['amount'].value * params.factor)",
          "params": {"factor": 1.1}
        }
      }
    },
    "aggs": {
      "regions": {
        "composite": {
          "size": 100,
          "sources": [{"region": {"terms": {"field": "region.keyword"}}}]
        },
        "aggs": {
          "total": {"sum": {"field": "amount_with_tax"}}
        }
      }
    }
  }
}
```

Nearby hostile fixtures must reject `operation: update_by_query` and a
terms-lookup source outside allowed scope, without rejecting the runtime script
or aggregation. Pipeline aggregation, vector search and language examples also
require their own positive fixtures under the acceptance plan below.

## Security and trust boundaries — required

Trusted: operator configuration, dedicated credential provisioning, pinned
binary/client/REST/grammar profiles and the administered Elasticsearch build.
Untrusted: every MCP field, server data, rendered templates, cursor strings,
client identity metadata and all query text. Returned document text remains
data even if it contains instructions.

Runtime authority starts with scoped `read` and `view_index_metadata`; optional
read-only cluster capabilities must be separately justified. Reject effective
document/index/configuration/security write capabilities, `run_as`, broad
administration, credential creation, and unknown action privileges. Examine
cluster, index, global/application, remote and restricted-index authority, not
just named roles. Elasticsearch privileges and endpoint policy are independent
layers. See [native privilege definitions](https://www.elastic.co/docs/reference/elasticsearch/security-privileges).

For users, inspect effective self privileges, not just assigned role names.
For API keys, bind the authenticated key ID, expiration/invalidation state,
assigned role descriptors and limiting descriptors to the proof; calculate
their effective intersection, including restrictions. Empty assigned
descriptors must not be treated as no privileges. Self-introspection support
is an exact-version acceptance gate. Where complete proof requires another
identity, use a separately configured read-only attestor with only required
security metadata access, never a query fallback credential. Do not grant
`manage_own_api_key` just to make inspection convenient. Metadata/errors from
the attestor are not exposed to callers. Missing proof fails startup.
See [self privilege inspection](https://www.elastic.co/docs/api/doc/elasticsearch/operation/operation-security-get-user-privileges)
and [API-key descriptor inspection](https://www.elastic.co/docs/api/doc/elasticsearch/operation/operation-security-get-api-key).

Use only configured HTTPS endpoints and explicit operator proxy settings.
Disable implicit environment proxy routing, redirects and automatic sniffing.
Check each endpoint's TLS identity and cluster UUID before using it; a node
advertisement does not authorize a new destination. Caller headers cannot
override Authorization, Host, run-as or opaque task identity. Cross-cluster
search is admitted only for configured aliases with both sides' proof; no
wildcard remote aliases bypass the configured scope.
See [cross-cluster security](https://www.elastic.co/docs/explore-analyze/cross-cluster-search).

| Hostile/ambiguous input | Rejecting layer |
| --- | --- |
| Encoded slashes, path traversal, CRLF, alternate method or host | Structured input and fixed-route encoder |
| Duplicate JSON keys, extra JSON/NDJSON records, invalid numbers | Bounded strict parser before dispatch |
| Update/delete hidden in a batch or query endpoint choice | Operation catalog and per-member validation |
| Lookup index hidden deep in DSL or language source | Complete source visitor/resolver and native grants |
| Stored template changes between proof and execution | Execute validated rendered bytes |
| Script writes request-local state | Allowed certified context; no keyword rejection |
| Custom script/plugin performs external I/O | Capability profile and egress boundary |
| Foreign PIT/cursor ID or mass clear | Context ownership registry |
| Security disabled, elevated API key or stale proof | Startup/re-attestation fails closed |

## Latency, admission, and pool budget — required

Inherit the [common agent workload contract](ARCHITECTURE.md#agent-workload-latency-budget).
Numbers below are proposed ES controls; they do not alter current defaults.
The normal configured request maximum is 10 minutes; operators may raise it to
the existing compiled 15-minute ceiling. Phase maxima never extend the parent.

| Boundary | Default | Configurable maximum | Hard ceiling | Budget includes | Cancellation and recovery |
| --- | ---: | ---: | ---: | --- | --- |
| Whole MCP request | 5m | 10m normal; up to 15m | 15m | Admission, proof, resolution, execution, collection, normal cleanup | Cancel transport; bounded recovery below |
| Admission queue | 1m | 5m | 5m, within parent | Global/target/workload and memory admission | Remove waiter promptly |
| Batch | 5m shared | Same as request | 15m shared | All members and PIT lifecycle | No deadline renewal per member |
| Connection acquisition | Remaining request | Remaining request | 15m | Socket/stream permit waits | Context-aware wait |
| Connect / TLS / discovery | 15s per phase | 1m | 1m, within parent | DNS, TCP, TLS, endpoint verification | Close failed connections; bounded attempts |
| Driver write | 15s | 1m | 1m, within parent | Sending bounded body | Close uncertain connection |
| Driver read / response headers | max request + 1m (11m) | 16m | 16m | Waiting for execution and streamed response | MCP context wins first |
| Server search / language execution | Remaining request | Remaining request | 15m | Endpoint-supported execution timeout | Also cancel HTTP; server timeout alone is insufficient |
| Cleanup / pool drain | 5s | 30s | 30s | Owned-context release and connection disposal | Separate bounded recovery context if parent has expired |

Agent reasoning holds no execution permit, connection or active query; an
explicit pagination handle may retain a separately budgeted server context.
Every active call starts its deadline before admission. Normal cleanup uses
the remaining deadline. After cancellation, return by the MCP deadline and
schedule bounded recovery under the maintenance budget; expose pending cleanup
and do not count uncertain server work as recovered. Repeated cleanup or server
cancellation failures quarantine the target to stop amplification.

For HTTP/1.1 use a target-wide connection budget shared by all endpoint pools,
covering dialing, active and idle sockets. `MaxConnsPerHost` alone does not bound
a multi-node target. Wrap `DialContext` with a context-aware global socket lease;
release exactly once on connection close. Enforce target-wide idle count with a
managed transport, plus max lifetime/idle time without interrupting active work.
Reserve maintenance scheduling capacity without creating an uncounted pool.
If HTTP/2 is enabled in a certified profile, additionally bound active streams;
socket limits are not concurrency limits. Initial transport qualification may
use HTTP/1.1. Proxy/load-balancer request timeouts must meet the same budget.

Disable SDK automatic retries and discovery; its documented defaults include
retries that must not silently multiply work. Permit at most one adapter-owned
retry (default zero) only for demonstrably safe pre-execution failures or a
certified stateless read after prior work has ended. Never replay ambiguous
cursor advancement, PIT creation or a timed-out batch. Retries/backoff consume
the original deadline. See [Go client configuration](https://www.elastic.co/docs/reference/elasticsearch/clients/go/configuration).

Propagate context cancellation to every request and close/reset the affected
HTTP exchange. Verify server-side cancellation on each enabled endpoint and
proxy profile using an independent test observer. Search timeout/cancellation
is cooperative; a client timeout is not proof that shard tasks stopped.
Do not grant runtime cluster-wide task cancellation to fix this. An endpoint
whose work cannot be bounded and observed remains an explicit release gate.
Short connect/write/cleanup defaults are phase limits inherited from the common
profile; they must not become a total search timeout.

## Resource and response budget — required

All byte sizes include decoded payloads. Request limits are checked before
constructing large object graphs; response limits apply while streaming, after
decompression, and again to the final encoded MCP result.

| Control | Default | Configurable hard ceiling / rule |
| --- | ---: | --- |
| Global / per-target active calls | 16 / 8 | 32 / global |
| Batch / maintenance concurrency | 4 / 2 | Within shared global and target ceilings |
| Metadata reservation | Common scheduler setting | Must leave interactive capacity |
| Queued requests | 256 | 1,024; additionally bounded by queued-byte budget |
| Connections open / idle per target | 8 / 4 | 64 / open; combined startup capacity validation |
| Hits or tabular rows per response | 5,000 | 10,000; native pagination for larger data sets |
| Final MCP response bytes | 8 MiB | 16 MiB, including envelope and any duplicated encoding |
| Cell / scalar bytes | 1 MiB | At most response limit |
| Native JSON request bytes, entire batch | 1 MiB | 16 MiB |
| Language text bytes | 256 KiB | 1 MiB; use existing SQL-text ceiling |
| Parameters | 1,000 | 10,000; native bindings |
| Parameter bytes / individual value | 8 MiB / 2 MiB | 64 MiB / 16 MiB; enclosing request limit also applies |
| Batch members | 50 | 100 |
| JSON depth / parsed nodes | 128 / 100,000 | 512 / 1,000,000; iterative bounded traversal |
| Resolved source indices per call | 1,024 | 10,000; explicit scope resource error on overflow |
| Returned aggregation buckets | 10,000 | 65,536 and response bytes; account for server's own limit separately |
| Configured endpoints per target | 8 maximum by default | 32; no auto-discovery expansion |
| Remote aliases per target | 0 enabled | 16 explicitly configured |
| Open contexts per target / process | 32 / 128 | 128 / 512; reserve capacity before server creation |
| Context idle lease / absolute lifetime | 5m / 30m | 15m / 2h; renewal cannot exceed absolute lifetime |
| Handle/token bytes | 64 KiB | 1 MiB; never log raw tokens |
| Metadata cache bytes / TTL per target | 16 MiB / 1m | 64 MiB / 5m; never substitutes for authorization |
| Query-result cache | Disabled | Future opt-in only after full semantic cache-key proof |
| Adapter retries | 0 | 1, subject to lifecycle rules |
| Forecast memory / queued bytes per process | 512 MiB / 64 MiB | 2 GiB / 256 MiB; bounded rejection before allocation |

Implementation must measure an allocation multiplier `A` per codec/parser
profile and reserve at least `A * (request bytes + decoded response budget)`
plus actual context/cache overhead before dispatch. Forecast the sum across
active calls, queued payloads, batch buffers, metadata caches and handle state;
reject impossible configurations. This bounds adapter work, not arbitrary
Elasticsearch cluster heap. Release qualification must demonstrate the measured
RSS fits the forecast and no unbounded decompression or aggregation buffering.

Do not inject `terminate_after`, disable expensive queries, shorten query DSL,
lower tracking accuracy or reduce aggregation `size` without an explicit caller
choice. If requested hits exceed the page cap, reject with the cap and a
pagination path. Composite aggregation pages may be requested explicitly;
arbitrary aggregation trees cannot safely be auto-paginated or truncated.
Return `resource_limit` rather than a syntactically valid but false aggregate.
Oversized scalar/document values fail by default; optional hit truncation must
mark precise paths and preserve IDs, sort keys and continuation correctness.
Never split an EQL sequence or truncate a sort/continuation token.

Default to failing on shard/time partial results. An explicit
`allow_partial_results` capability may expose native partial data with reasons;
never report it as a complete answer. Honor native approximation metadata even
when `partial=false`. Operator-set ES circuit breakers, expensive-query
settings, result windows and language row limits remain visible constraints;
diagnose them without pretending the MCP imposed a mutation restriction.

## Compatibility and migration

Additive tools and target metadata only. Existing SQL/Redis requests, defaults
and configurations retain their behavior. The implementation must update
`internal/config`, `configs/example.yaml`, strict config tests, README,
SECURITY and ARCHITECTURE together; this RFC-only change deliberately does not
put unsupported keys in the runnable example configuration.

Missing version/license features produce capability errors scoped to that
feature. Never automatically fall back from Elasticsearch to OpenSearch, from
ES|QL to a simplified DSL, from PIT to live search, or from a denied principal
to an attestor. Rollback removes/disables ES targets and closes their owned
contexts; it requires no business-data migration. Tighter timeout/pool profiles
are availability changes and require measurements and operator overrides.

## Observability

Record content-free metrics for admission/queue latency, endpoint operation,
execution/collection latency, pool waits, socket/stream counts, cancellation
requested/observed, cleanup failures, active/expired contexts, proof freshness,
retry attempts, partial/truncated outcomes and memory reservations. Audit a
query ID, target alias, profile revision, operation and keyed query fingerprint.
Do not log query bodies, script/template text, parameters, returned documents,
raw server error bodies, URLs, credentials or cursor IDs. Use bounded labels;
index names, query IDs and fingerprints are not metric labels.

Separate policy rejection, server permission denial, unsupported feature,
resource exhaustion and cancellation failures. Sanitize native errors because
they can echo query fragments, document values and infrastructure addresses.
Dashboards must distinguish client return from confirmed server recovery.

## Test and acceptance plan — required

1. Config/contracts: five-minute default, ten-minute normal maximum, fifteen-
   minute ceiling; all phase/pool bounds, conflicting auth/engine blocks,
   target mismatch, precise numbers, duplicate JSON keys and native result shapes.
2. Authority: native/file users and API keys, descriptor intersection, empty
   descriptors, expired/invalidated keys, roles/actions with hidden write powers,
   run-as, remote/application grants, restricted indices, missing introspection,
   stale proof, rotation and drift. Never run startup mutation probes on real
   targets. In disposable fixtures, bypass adapter validation and prove the
   runtime credential itself cannot persist writes.
3. Source escapes: all embedded source families, deeply nested bool/aggregation
   bodies, SQL subqueries, ES|QL LOOKUP JOIN/ENRICH/views, multi-search headers,
   date math, wildcards, alias routing/filters, data-stream rollover, hidden
   backing indices, direct-get alias behavior, DLS/FLS and remote aliases.
4. Advanced positives: nested/parent-child/full-text/geo/percolate search,
   regexp and query-string options, pipeline/composite/scripted aggregations,
   inline/stored scripts with local assignments, runtime fields/lookups,
   templates with adversarial parameter strings, vector/hybrid queries,
   SQL expressions/parameters, ES|QL joins and EQL sequences. Pair each with
   mutation/scope-escape negatives; no blanket script/join/expensive-query ban.
5. Lifecycle: PIT/search_after, sliced scroll, composite pagination, SQL cursor,
   rotated/foreign/replayed IDs, concurrent next/close, timeout after server
   creation but before ID delivery, expiry, lost connection, process crash and
   proof drift. Unknown IDs after lost responses expire within bounded server
   leases; no ambiguous retry creates unbounded orphan contexts.
6. Latency/recovery: real multi-minute success below default and configured
   deadlines, deadline cancellation, shard-task disappearance observed by a
   separate test identity, permit recovery, pool reuse/disposal, slow DNS/TLS,
   writes/headers/reads, proxy disconnect, node loss and cleanup failure.
7. Saturation: interactive, metadata, batch and maintenance together; shared
   batch deadline and partial-member status; hard socket/stream counts across
   endpoints, idle eviction/lifetime, cancellation storms and retry budgets.
8. Resource/quality: fuzz route/JSON/source parsers, race tests, soak and RSS,
   decompression bombs, many contexts, oversized hits/cells/aggregation trees,
   exact MCP encoded size, preserved large integers, approximation/partial
   flags and unchanged aggregation/sort semantics under budget pressure.
9. Version qualification: pinned 8.19.x and selected 9.x builds/client versions,
   licensed feature profiles, a tested rolling-upgrade pair and configured CCS.
   Test fixture images and licenses must be reproducible. A skip is not a pass.

Document the exact successful commands/builds and observed limits when each
implementation phase lands. The linked qualification record distinguishes fixture results from live-server
evidence; a missing integration environment is an explicit skip.

## Rollout and operations

1. Implement config, transport, attestation, operation catalog and disposable
   hostile fixtures. Validate target-wide socket and memory accounting first.
2. Deliver native DSL/metadata/document reads with advanced scripts, aggregation,
   templates and vector positives. Label this a partial implementation.
3. Deliver context lifecycles/batches and language parsers/tools, scope visitors,
   ENRICH and configured cross-cluster profiles. Publish per-profile capability
   and license requirements. Keep unfulfilled mandatory gates visible.
4. Run real cancellation/saturation/soak/version matrices; provision read-only
   reporting targets with verified TLS and capacity for the common budgets.
5. Canary with explicit target aliases; monitor task recovery, context counts,
   shard pressure and proof health before broader use. Roll back on unexplained
   privilege drift, persistent task/context leaks or memory-budget violations.

Operators may lower workload budgets or disable a costly capability on their
own cluster, but defaults must not substitute a feature ban for implementation.
Querying replicas/shards does not provide a dedicated analytics resource pool;
use a separately provisioned reporting cluster when isolation is required.

## Alternatives considered

- GET-only proxy: misses valid POST reads and does not classify endpoint effects.
- Unrestricted proxy plus read credentials: permits scope/lifecycle misuse and
  cannot explain native operations with persistent auxiliary effects.
- Tiny term/match DSL or disabling scripts, joins, vectors and aggregations:
  contradicts the requirement to preserve advanced reads.
- Regex-based SQL/ES|QL checks: cannot resolve source references and extensions.
- Convert every result into SQL rows: loses aggregation, sequence and pagination
  semantics and creates misleading results.
- Reject all context creation/DELETE calls: conflates temporary search-resource
  cleanup with durable data deletion and prevents deep pagination.
- Enable stored async results because ES calls the privilege `read`: ignores
  actual persistence and silently changes the security objective.
- Unlimited SDK retries/sniffing or per-host pools: breaks target, deadline and
  aggregate connection guarantees.

## Completion criteria

- [ ] Draft reviewed for endpoint effects, privileges, source scope and budgets.
- [ ] Elasticsearch config, registry, native core capabilities and MCP tools land.
- [ ] Every mandatory advanced query family above has passing positive and
  hostile tests on its supported profile; simple search alone is insufficient.
- [ ] Language parser/helper choice and deployment are implemented and tested.
- [ ] Runtime user/API-key proof and bounded re-attestation work without writes.
- [ ] Pagination, batch consistency, cancellation and cleanup are proven on real
  servers with hard connection/memory/response bounds.
- [ ] Exact version/license/CCS matrices pass; remaining optional inference,
  plugin or async effect profiles are explicitly documented.
- [ ] Example configuration, architecture, security policy, operational guidance
  and measured release results agree with actual implemented capabilities.
- [ ] Canary exit gates pass; ES support is advertised only to that proven scope.

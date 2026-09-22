# Elasticsearch native query/batch implementation evidence — 2026-09-22

This record supersedes the phase 1 query-availability statements. It does **not**
certify a real Elasticsearch deployment. The configured build pins remain
8.19.21 / `4fe44c255c3d0da06779b921e132cdc555ed9aff` and
9.1.10 / `f2e019bd8110088070638ca779ec1543188c0f43`.

## Delivered interface

- `es_metadata`: existing resolve/mappings behavior.
- `es_query`: search, count, get, mget, termvectors, mtermvectors, explain,
  field_caps, search_shards, indices.validate_query, search_template and
  render_search_template. Document IDs are structured `id` values, escaped once.
- `es_batch`: typed members, `independent` consistency, one deadline and combined
  output budget. All members are proved before executable query dispatch.
  Compatible searches use service-generated `_msearch` NDJSON with native search
  concurrency one. Other operations/options execute sequentially with their
  original semantics. A batch error fails the tool instead of returning unlabeled
  incomplete results; completed members are reads and need no rollback.

Search retains native bodies: bool/full-text, nested and parent/child, span and
intervals, geo, percolate, more-like-this, scoring, highlighting, suggestions,
search_after, collapse/inner_hits, native aggregations, composite after_key,
pipeline/scripted_metric calculations, runtime fields/lookups, supplied-vector
kNN/sparse vectors, script scoring and local retriever fusion/rescoring.
There is no replacement term/match-only query language or keyword ban on scripts.
Unavailable parser/profile cases return `capability_unavailable`.

Native response JSON preserves large integers, sort keys, total-hit relation,
approximation metadata, aggregation results and continuation values. Document
reads report realtime consistency unless explicitly disabled; ordinary searches
report eventual consistency. Native timeout/shard/early-termination partials fail.
No automatic terminate_after, approximate tracking or aggregation-size reduction
is inserted. `max_rows` bounds total returned outer/inner/top hits; aggregation
buckets use `max_aggregation_buckets` (default 10,000; maximum 65,536). Encoded MCP
text and structured copies share `max_result_bytes`.

## Proof and transport details

1. Existing realm privilege and index-pattern containment proofs remain active.
   Query preflight additionally reads `_nodes/plugins`: every node must report
   the pinned build and an empty external-plugin inventory. Bundled modules are
   part of that build; external analyzers, field types and script engines need
   explicit future effect profiles. Metadata still works when a query plugin
   profile is unavailable.
2. Contextual visitors resolve terms lookup, indexed shapes (including the native
   default `shapes` index), percolator/MLT/pinned documents, runtime lookup targets,
   nested sorts, filter/background-filter aggregations and wrapped base64 queries.
   Caller field names, artificial documents, script parameters and aggregation
   metadata are data, not executable control fields.
   Source inventories are resolved again before data is returned, so observed
   alias/rollover drift also rejects aggregate-only results without hit indices.
   An administrator's change-and-restore between observations is not detected;
   this is not an atomic server-side scope lock.
3. Multi-term-vector `parameters` are materialized into effective documents in
   native input order before source proof. This matters because the pinned
   [native parser](https://github.com/elastic/elasticsearch/blob/4fe44c255c3d0da06779b921e132cdc555ed9aff/server/src/main/java/org/elasticsearch/action/termvectors/MultiTermVectorsRequest.java)
   applies defaults to subsequent `docs`, and final defaults to `ids`.
4. Inline Painless/expression scripts retain request-local calculations. Stored
   script definitions are fetched and frozen inline before dispatch. Their
   inspection accepts the read action `cluster:admin/script/get`; `manage`,
   `script/*` and script writes remain denied. The pinned
   [privilege resolver](https://github.com/elastic/elasticsearch/blob/4fe44c255c3d0da06779b921e132cdc555ed9aff/x-pack/plugin/core/src/main/java/org/elasticsearch/xpack/core/security/authz/privilege/ClusterPrivilegeResolver.java)
   accepts action names and expands their action prefix. Provisioning and live
   privilege-action coverage remain qualification gates.
5. Mustache templates are rendered, decoded, visited and executed as the inspected
   ordinary search body. Mutable template IDs are never re-executed after proof.
   Stored templates require readable Mustache definitions. See the native
   [render API](https://www.elastic.co/docs/api/doc/elasticsearch/operation/operation-render-search-template).
6. Mappings are checked for implicit semantic_text inference, including field
   aliases. Ordinary fields on mixed mappings remain usable. Query-string
   expressions currently require the selected mappings to contain no semantic
   fields, until a separate native field-expression proof is delivered. Explicit
   inference builders/rerankers remain a distinct unavailable capability;
   supplied vectors require no inference service.
7. Native options come from pinned REST specifications. Raw URL/header/source
   injection, unowned scroll/PIT IDs and partial-result opt-in are unavailable.
   Document `refresh=true` is denied as a visibility mutation. Caller native
   search timeouts are capped by the remaining request deadline, including batch
   preparation. HTTP context cancellation releases adapter resources.

## Verification

The TLS fixtures cover both pinned version identities and exercise advanced
search bodies, scripts with local assignments, runtime lookup sources, vectors,
fusion, frozen templates/scripts, native large numbers, alias routing, document
IDs containing reserved URL characters and missing documents. Negative cases
cover nested source escapes, wrapped DSL, template output escapes, inference,
plugins, mutation operations, refresh, raw contexts, duplicate JSON, shard/time
partials, oversized output and bad batch members before dispatch.

MCP tests call the registered query and batch tools through the SDK transport and
verify exact large integers plus strict raw argument handling. Cancellation tests
consume the HTTP request body before waiting for disconnect and verify both
permit release and a successful subsequent query. This demonstrates fixture
transport recovery, not real ES task cancellation.

Validation commands:

```sh
go test -race ./... -count=1
go vet ./...
make build
python3 tools/es-rest-catalog/generate.py --cache /tmp/readonly-es-rest-specs --check
```

The opt-in `TestElasticsearchLiveReadOnlyMetadata` test now also runs native
search/script/runtime-field/batch positives and the existing native write-denial
probe. It requires an operator-provisioned disposable index and read-only account
via `READONLY_DB_MCP_ES_CONFIG`, `READONLY_DB_MCP_ES_TARGET`, and
`READONLY_DB_MCP_ES_WRITE_PROBE_INDEX`. It was **not run** for this record: no ES
service/account was provisioned, Java was unavailable, and the installed Docker
launcher reported that WSL integration was not enabled.

## Remaining implementation and qualification gates

- Owned PIT/scroll/cursors and PIT batch consistency; SQL/ES|QL/EQL parsers;
  API-key/attestor profiles; date-math and cross-cluster sources.
- Dynamic suggestion collate templates and stored query-rule expansion; separate
  source/exists convenience operations and remaining native/profile variations.
  Custom plugins/inference need effect, authority, egress and cost proofs.
- Real builds, native script/template permissions and inline materialization
  equivalence, licensed fusion features, alias/rollover and mapping drift, actual
  server task cancellation and native write-denial acceptance.
- Sustained mixed load, measured codec allocation multipliers and peak RSS,
  MCP delivery backlog, proxy/failover and production TLS/topology matrices.

Administrators can change aliases, mappings, stored scripts and nodes between
inspection and execution. Preflight is not an atomic server snapshot. Strict
native grants and controlled provisioning remain required; build metadata is
not cryptographic attestation. Full RFC-0006 completion is not claimed.

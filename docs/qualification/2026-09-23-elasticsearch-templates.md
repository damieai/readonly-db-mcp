# Elasticsearch template diagnostics and batches — 2026-09-23

Status: implementation and TLS/MCP fixture evidence for pinned 8.19.21 and
9.1.10. A live read-only qualification test is available but has not run against
a real Elasticsearch deployment in this environment.

`search_template` and `render_search_template` accept native inline or stored
Mustache templates and parameters. `explain` and `profile` are boolean envelope
controls. Native template execution defaults both to false and applies them
after rendering, overriding fields of the same name in the rendered search.
The REST `options.explain` override remains available and retains its native
precedence. Render-only simulation returns the validated rendered fields before
execution flags are applied. Search text, parameters, scripts, aggregations,
runtime fields and JSON number precision still pass through the existing
effect/source visitor. Stored scripts and templates are inspected and frozen.

`es_batch` accepts multiple `search_template` members, including distinct
templates, parameters, scopes, routing and diagnostic flags. All members render
and pass proof before execution. Compatible members use service-built NDJSON on
`/_msearch`; other combinations execute individually under the same batch lease
and deadline so their options are retained. A shared PIT batch still uses its
owned PIT lifecycle. The pinned native `/_search/template` and
`/_msearch/template` execution handlers use ordinary listeners without binding
the task to HTTP channel closure; both pinned `/_msearch` handlers use
`RestCancellableNodeClient`. Executing the frozen search bodies through the
cancellable routes also prevents a second Mustache expansion when a rendered
literal contains `{{...}}`. Raw `msearch_template` is not a public `es_query`
operation; `inspect_target` identifies it as `implemented_via_es_batch`.

The generated operation catalog is sourced from the pinned REST specifications.
Native behavior was checked against both pinned
[TransportSearchTemplateAction](https://github.com/elastic/elasticsearch/blob/4fe44c255c3d0da06779b921e132cdc555ed9aff/modules/lang-mustache/src/main/java/org/elasticsearch/script/mustache/TransportSearchTemplateAction.java),
[SearchTemplateRequest](https://github.com/elastic/elasticsearch/blob/4fe44c255c3d0da06779b921e132cdc555ed9aff/modules/lang-mustache/src/main/java/org/elasticsearch/script/mustache/SearchTemplateRequest.java),
[RestMultiSearchTemplateAction](https://github.com/elastic/elasticsearch/blob/4fe44c255c3d0da06779b921e132cdc555ed9aff/modules/lang-mustache/src/main/java/org/elasticsearch/script/mustache/RestMultiSearchTemplateAction.java)
and [RestMultiSearchAction](https://github.com/elastic/elasticsearch/blob/4fe44c255c3d0da06779b921e132cdc555ed9aff/server/src/main/java/org/elasticsearch/rest/action/search/RestMultiSearchAction.java).
The corresponding 9.1.10 source commit is
`f2e019bd8110088070638ca779ec1543188c0f43`.

Fixture tests cover both pins, single search and simulation control precedence,
type rejection before rendering, stored-definition freezing, member-specific
controls and routing, literal Mustache markers, large integer parameters,
native batch framing, whole-batch failure for a denied source or malformed
result, fallback for options with no batch-header equivalent, MCP calls and
execution channel cancellation. None of those fixtures run the native Mustache
or search engine.

To run the live read-only test, provision an existing test index named
`mcp-es-acceptance-*` with at least one document and set `READONLY_DB_MCP_ES_CONFIG`,
`READONLY_DB_MCP_ES_TARGET`, and `READONLY_DB_MCP_ES_INDEX`. The target must use
`environment: test` and include the index in its configured read scope. Run
`go test ./internal/dialects/elasticsearch -run TestElasticsearchLiveTemplates -v`.
The test checks single and batched Mustache rendering, and actual profile and
explanation output with both flag values; it does not create data or templates.
Missing settings result in an explicit skip. Real-server checks are still needed
for stored-template privilege grants, multiple nodes, PIT templates, concurrent
template changes, task cancellation and saturation on both pinned versions.

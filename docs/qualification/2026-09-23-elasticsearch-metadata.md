# Elasticsearch alias and settings metadata — 2026-09-23

Status: implementation with pinned 8.19.21 and 9.1.10 TLS/MCP fixture evidence.
The optional live read-only test has not run against a real Elasticsearch
deployment in this environment.

`es_metadata.operation=aliases` uses `GET /<indices>/_alias` and
`operation=settings` uses `GET /<indices>/_settings`. The requests preserve
native index, alias, data-stream and date-math expressions. The pinned REST
operation catalog supplies each version's query options, including
`ignore_unavailable`, `allow_no_indices`, `expand_wildcards`, `flat_settings`
and `include_defaults` where supported. For example:

```json
{"target":"search-reporting","operation":"settings","indices":["reports-*"],"options":{"flat_settings":true,"include_defaults":true}}
```

Before the native metadata call, `_resolve/index` checks every resulting
index, alias and data stream against configured scope. Every returned index
key must be in that physical inventory, except a native data-stream alias entry
may use its already-proved logical stream name. Returned alias names must also
satisfy the configured scope. Settings for data streams use their backing-index
keys. Duplicate JSON keys and malformed response shapes are
rejected. The inventory is resolved again before returning a result, so an
observed rollover during the call fails whole. Existing admission, deadline,
response-size and TLS/build/permission checks apply. The dedicated identity
needs `view_index_metadata`, already required for mappings.

Native route, option and authorization behavior was checked against the pinned
REST specifications and the [Get aliases API](https://www.elastic.co/docs/api/doc/elasticsearch/operation/operation-indices-get-alias)
and [Get index settings API](https://www.elastic.co/docs/api/doc/elasticsearch/operation/operation-indices-get-settings).
Fixtures cover both pins, option forwarding, date-math paths, data-stream alias
and settings keys, scope and alias escape, malformed or duplicate responses,
inventory drift, whole-response denial and MCP calls.
Fixtures do not run a native Elasticsearch server.

For live acceptance, provision an existing `mcp-es-acceptance-*` index with an
existing `mcp-es-acceptance-*` alias. Set `READONLY_DB_MCP_ES_CONFIG` to a config
with an `environment: test` Elasticsearch target, `READONLY_DB_MCP_ES_TARGET`
to that target name, `READONLY_DB_MCP_ES_METADATA_INDEX` to the index and
`READONLY_DB_MCP_ES_METADATA_ALIAS` to the alias. Run
`go test ./internal/dialects/elasticsearch -run TestElasticsearchLiveAliasSettingsMetadata -v`.
The test reads alias definitions and settings (including defaults); it creates
or changes nothing. Missing settings cause an explicit skip. Real-server
multi-node, alias-filter and concurrent-rollover checks remain outstanding.

The separate read-only `_rank_eval` API was reviewed but remains unavailable.
At both pinned commits,
[the REST handler](https://github.com/elastic/elasticsearch/blob/4fe44c255c3d0da06779b921e132cdc555ed9aff/modules/rank-eval/src/main/java/org/elasticsearch/index/rankeval/RestRankEvalAction.java)
uses `NodeClient.executeLocally` without `RestCancellableNodeClient`; closing a
client connection does not provide the task-lifetime proof used by the current
query adapter. A bounded, cancellable execution profile is needed before
exposing it through `es_query`.

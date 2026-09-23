# Elasticsearch document source and existence reads — 2026-09-23

Status: implemented with pinned 8.19.21 and 9.1.10 TLS fixture evidence.
The optional live read-only test has not run against an Elasticsearch deployment
in this environment.

`es_query` and independent `es_batch` accept `exists`, `get_source`, and
`exists_source`, respectively using native `HEAD /<index>/_doc/<id>`,
`GET /<index>/_source/<id>`, and `HEAD /<index>/_source/<id>` routes. The
structured `id` field is URI-escaped; one concrete index or alias is required,
and pinned native options such as `routing`, `realtime`, and source filtering
remain available. As with `get`, `refresh=true` is rejected because it changes
index visibility. For example:

```json
{"target":"search-reporting","operation":"get_source","indices":["reports-2026"],"id":"doc-1","options":{"_source_includes":["title","event.*"]}}
```

Elasticsearch HEAD responses have no JSON body. The tool represents status 200
as `{"exists":true}` and 404 as `{"exists":false}`; other statuses fail. A
missing `get_source` remains an upstream error. A successful `get_source`
returns the exact JSON source, including user fields named `error`, `pit_id` or
`_scroll_id`. Its native response has no `_index` field, so the adapter relies
on scoped preflight resolution, a post-execution inventory recheck and the
dedicated account's native index permissions; it cannot infer a physical index
from source contents. Requests and responses retain their existing size,
deadline and admission bounds.

The routes and native options were checked against Elasticsearch's
[document existence](https://www.elastic.co/docs/api/doc/elasticsearch/operation/operation-exists),
[source retrieval](https://www.elastic.co/docs/api/doc/elasticsearch/operation/operation-get-source),
and [source existence](https://www.elastic.co/docs/api/doc/elasticsearch/operation/operation-exists-source)
API references. Fixtures cover both pinned versions, encoded IDs, routing,
realtime mode, HEAD status handling, opaque source fields, batch forwarding,
denied refresh, out-of-scope indices and observed inventory drift.

For optional live acceptance, set `READONLY_DB_MCP_ES_CONFIG` and
`READONLY_DB_MCP_ES_TARGET` to a read-only `environment: test` target,
`READONLY_DB_MCP_ES_DOCUMENT_INDEX` to an existing `mcp-es-acceptance-*`
index, and `READONLY_DB_MCP_ES_DOCUMENT_ID` to an existing document with an
enabled source. Run
`go test ./internal/dialects/elasticsearch -run TestElasticsearchLiveDocumentReads -v`.
The test performs only these three reads. Real-server acceptance remains
outstanding in this environment.

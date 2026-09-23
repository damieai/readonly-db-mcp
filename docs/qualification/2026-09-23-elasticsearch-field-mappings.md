# Elasticsearch selected field mappings — 2026-09-23

Status: implemented with pinned 8.19.21 and 9.1.10 TLS/MCP fixture evidence.
The optional live read-only test has not run against an Elasticsearch deployment
in this environment.

`es_metadata.operation=field_mappings` uses the pinned native
`GET /<indices>/_mapping/field/<fields>` route. The required `fields` array
accepts multiple field names and native wildcard expressions. `options` follows
each pinned REST specification; `mappings` also accepts its native options.
For example:

```json
{"target":"search-reporting","operation":"field_mappings","indices":["reports-*"],"fields":["event.*","user.id"],"options":{"ignore_unavailable":false}}
```

The request resolves every index, alias and data stream against the configured
scope before reading mappings. Returned physical index keys must be in that
inventory. Malformed field mapping entries, duplicate JSON keys, oversized
results, and observed inventory drift fail the whole response. The response
retains Elasticsearch's original field definitions and number precision. Native
`view_index_metadata` authority, TLS/build checks, admission, deadline and
request-size limits remain in force.

Pinned REST routes and options were checked against the
[8.19 Get field mapping API](https://www.elastic.co/guide/en/elasticsearch/reference/8.19/indices-get-field-mapping.html)
and [current API reference](https://www.elastic.co/docs/api/doc/elasticsearch/operation/operation-indices-get-field-mapping).
Fixtures cover both pinned builds, wildcard path encoding, native options,
response scope and shape rejection, inventory drift, and MCP forwarding.

For live acceptance, set the read-only acceptance variables described in
[the metadata qualification record](2026-09-23-elasticsearch-metadata.md), then
run `go test ./internal/dialects/elasticsearch -run TestElasticsearchLiveAliasSettingsMetadata -v`.
The test also reads wildcard field mappings from the existing acceptance index;
it creates or changes nothing. Real-server alias rollover and mapping-drift
tests remain outstanding.

# Elasticsearch stored-script metadata — 2026-09-23

Status: implemented with pinned 8.19.21 and 9.1.10 TLS fixture evidence.
Real-server acceptance has not run in this environment.

`es_metadata.operation=get_script` reads `GET /_scripts/{id}` for exactly one
ID in `names`. `elasticsearch.readable_script_ids` grants the exact IDs whose
definitions may be returned to MCP callers. No ID prefix, wildcard or script
language ban is imposed. This grant controls metadata disclosure, independently
of query execution through a stored script or template. For example:

```yaml
elasticsearch:
  readable_script_ids: [report-template]
```

```json
{"target":"search-reporting","operation":"get_script","names":["report-template"],"options":{"master_timeout":"2s"}}
```

The adapter checks the allowlist before dispatch, escapes the ID as one path
element, caps native `master_timeout` to the remaining request deadline, and
requires a matching `_id`, `found: true` and a native script definition in the
response. Duplicate JSON keys, excessive JSON depth/size and oversized MCP
results fail whole. Script sources may be strings or native template objects;
large integers remain intact. The narrow `cluster:admin/script/get` grant is
already accepted by the query proof profile; script management privileges are
not needed. See the pinned [8.19 REST route](https://github.com/elastic/elasticsearch/blob/4fe44c255c3d0da06779b921e132cdc555ed9aff/rest-api-spec/src/main/resources/rest-api-spec/api/get_script.json)
and [9.1 REST route](https://github.com/elastic/elasticsearch/blob/f2e019bd8110088070638ca779ec1543188c0f43/rest-api-spec/src/main/resources/rest-api-spec/api/get_script.json).

Fixtures cover both versions, MCP forwarding, path escaping, exact ID scope,
response identity/shape, native number precision, deadline bounding and
admission release. This is fixture evidence, not proof that a deployed ES role
grants the exact action or that live templates match expected definitions.
For live acceptance, configure `READONLY_DB_MCP_ES_CONFIG` and
`READONLY_DB_MCP_ES_TARGET` for an `environment: test` target, set
`READONLY_DB_MCP_ES_SCRIPT_ID` to an existing allowlisted ID, then run
`go test ./internal/dialects/elasticsearch -run TestElasticsearchLiveStoredScriptMetadata -v`.
The test only reads the definition.

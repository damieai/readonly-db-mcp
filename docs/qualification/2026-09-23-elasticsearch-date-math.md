# Elasticsearch date math index sources — 2026-09-23

Status: implemented with TLS fixture and MCP tests against the pinned 8.19.21
and 9.1.10 profiles. The optional live read-only test has not run against a real
Elasticsearch deployment in this environment.

Native date math index names such as
`<reports-{now/d{yyyy.MM.dd|+08:00}}>` are accepted as index selectors for
`es_metadata`, scoped document/search operations, EQL, SQL, ES|QL, and nested
read-only index references. The service keeps the original expression for the
native request. It encodes delimiters, including `/`, `+` and `:`, in URL paths;
SQL and ES|QL retain their original query text. Elasticsearch performs date
evaluation on the pinned endpoint. Before execution, the service resolves the
selector and checks the resulting indices, alias names, data streams and backing
indices against configured allow/deny scope. It rechecks source resolution and
withholds the whole response when source inventory changes. Caller-supplied
index lists remain a separate narrowing constraint for SQL and ES|QL. Neither
configuration nor role privilege patterns acquire date-expression syntax.

The native syntax and URL-encoding rules come from the
[Elasticsearch API conventions](https://www.elastic.co/guide/en/elasticsearch/reference/8.19/api-conventions.html).
The [ES|QL FROM reference](https://www.elastic.co/docs/reference/query-languages/esql/commands/from)
and [SQL date/time reference](https://www.elastic.co/docs/reference/query-languages/sql/sql-functions-datetime)
describe date math sources in those languages. These references establish
native behavior; fixture tests establish only the adapter's routing and proof.

Tests cover both pinned versions, formatted timezone expressions, URL encoding,
metadata/mappings, search, EQL, SQL, ES|QL, nested terms lookup, alias logical
scope, requested-scope containment, route-escape rejection, out-of-scope
resolution before execution, whole-batch failure on resolution drift, and MCP
tool calls. The fixtures do not evaluate `now` or execute Elasticsearch's query
engines.

For live acceptance, provision an existing `mcp-es-acceptance-*` index selected
by a native date expression. Set `READONLY_DB_MCP_ES_CONFIG` to a config with a
`test` Elasticsearch target, `READONLY_DB_MCP_ES_TARGET` to its name,
`READONLY_DB_MCP_ES_DATE_SOURCE` to the expression, and
`READONLY_DB_MCP_ES_DATE_INDEX` to the expected concrete index. Run
`go test ./internal/dialects/elasticsearch -run TestElasticsearchLiveDateMath -v`.
The test reads resolution, mappings, search and count; it makes no writes.
Missing settings cause an explicit skip. Separate live certification on both
pinned versions, including timezone boundaries and rollover during a request,
remains outstanding.

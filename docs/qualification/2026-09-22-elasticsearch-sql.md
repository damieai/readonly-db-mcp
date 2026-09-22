# Elasticsearch native SQL qualification — 2026-09-22

Status: implementation, generated-parser and TLS/MCP fixture evidence. This
record does **not** certify a real Elasticsearch deployment. ES|QL, remote-index
execution and persisted async SQL/EQL remain separate pending profiles.

## Implemented surface

- `es_query.operation=sql.query`: original native SQL and scalar/typed parameters,
  complete bounded results collected across native pages under one deadline.
- `es_query.operation=sql.translate`: the original SQL sent to the native
  translation endpoint after the same source/effect preflight.
- `es_cursor.kind=sql`: open/next/close with a random session-owned handle,
  fixed query/page size/orientation, rotated native tokens and SQL-specific clear.
- Independent batches can mix SQL, EQL and other native reads; every member is
  preflighted before execution. Shared-PIT batches remain compatible searches.

The complete upstream grammars differ between 8.19.21 and 9.1.10 and are generated
into separate Go packages with ANTLR 4.13.1. They preserve Apache License 2.0 and
Presto-origin notices, independently of the EQL grammar's Elastic License 2.0.
Java is used only for reproducible generation, never at service runtime. See
[grammar manifests, packaging and reproduction](../../tools/es-language-parser/README.md).

Parsing covers expressions, functions, grouping, PIVOT, nested queries, CTEs,
JOINs, EXPLAIN/DEBUG and SHOW/SYS commands. This is **syntax/source coverage**;
the native server still decides function, plan and JOIN/CTE semantic availability.
In particular, the pinned native pre-analyzer inspects unresolved relation names
before CTE substitution. Source proof therefore includes CTE references as well
as their definitions; it never assumes every CTE-looking name is source-free.
A name outside the target/request index scope is not admitted solely because
it is also declared as a CTE. Original query text and parameters are not rewritten.

All relation branches and metadata selectors are checked, including quoted
identifiers and bound LIKE parameters. Native mapping resolution expands LIKE
`_` to `*`; proof uses that actual broader expansion. An omitted metadata index
selector is native `*`, not an implicit substitution of configured indices.
Use an explicit allowed selector such as `SHOW TABLES LIKE 'reports-%'`.
Catalog/type enumeration forms that do not read indices remain available.
Explicit catalog execution is limited to the verified local cluster name;
remote catalogs need a separate authority profile. The `::data` source selector
uses the normal local data inventory; failure-store selectors await their own
backing-index authority proof.

Alias names remain in the SQL. Mapping runtime lookups, request filter/runtime
fields and stored script definitions use existing native visitors. MATCH fields
(and possible alias-stripped paths) are checked for inference-backed mappings;
QUERY expressions require mapping-wide inference proof, as in the native DSL
query-string profile. Ordinary SQL expressions, scripts and literal strings do
not become index selectors or mutation keywords.

## Protocol and lifecycle boundaries

Both pinned transports choose persisted async execution only for a present,
non-negative `wait_for_completion_timeout`. This implementation accepts omission
or `-1`, rejects retained async results, sets `allow_partial_search_results=false`
and uses the cancellable REST channel. Native `request_timeout` is capped by the
remaining MCP deadline. The underlying query phase routes timeout handling through
`SearchTimeoutException.handleTimeout(allowPartialSearchResults, ...)` rather
than admitting timed-out shard results when partial results are disabled.

Structured JSON is fixed explicitly, including `binary_format=false` in driver
modes. Native mode/version/client controls and row/columnar output are preserved.
Columns appear on the initial page; subsequent page dimensions are checked
against their retained count. Large integers and cell values are never converted
through floating point. SQL `max_rows` means the whole result for `es_query`, or
one page for `es_cursor`. Explicit `fetch_size` cannot exceed that limit; omission
uses `min(1000, max_rows)`. Result overflows fail whole and clean owned state.

`page_timeout` or public `keep_alive_ms` selects the bounded native lease; both
cannot be specified together. Query and page size are fixed by cursor open.
Only output controls are retained locally for continuation; the native token
contains the frozen query. The server's lack of a continuation token closes the
handle even for a nonempty last page. No public request accepts raw SQL cursors,
async IDs, caller paths or clear-all. Native tokens are captured before validating
page data, stripped from public results and cleared only with `POST /_sql/close`.
An unexpected async ID/running result invalidates the target profile.

Session, target, process and authority ownership, serialized advance/close,
expiry, disconnect, shutdown and uncertain-outcome reservations reuse the existing
PIT/scroll registry. Creation and continuation are never retried. Failed clears
keep their slot/memory until successful cleanup or the conservative native lease
horizon. HTTP cancellation releases interactive admission before bounded recovery.
SQL uses the same 128 process slots / 64 MiB retained-context pool.

Protocol evidence checked at both immutable commits:

- [8.19.21 grammar](https://github.com/elastic/elasticsearch/blob/4fe44c255c3d0da06779b921e132cdc555ed9aff/x-pack/plugin/sql/src/main/antlr/SqlBase.g4)
  and [9.1.10 grammar](https://github.com/elastic/elasticsearch/blob/f2e019bd8110088070638ca779ec1543188c0f43/x-pack/plugin/sql/src/main/antlr/SqlBase.g4).
- [Request parsing](https://github.com/elastic/elasticsearch/blob/4fe44c255c3d0da06779b921e132cdc555ed9aff/x-pack/plugin/sql/sql-action/src/main/java/org/elasticsearch/xpack/sql/action/SqlQueryRequest.java)
  and [shared request fields](https://github.com/elastic/elasticsearch/blob/f2e019bd8110088070638ca779ec1543188c0f43/x-pack/plugin/sql/sql-action/src/main/java/org/elasticsearch/xpack/sql/action/AbstractSqlQueryRequest.java).
- [SQL transport dispatch](https://github.com/elastic/elasticsearch/blob/4fe44c255c3d0da06779b921e132cdc555ed9aff/x-pack/plugin/sql/src/main/java/org/elasticsearch/xpack/sql/plugin/TransportSqlQueryAction.java),
  [response encoding](https://github.com/elastic/elasticsearch/blob/f2e019bd8110088070638ca779ec1543188c0f43/x-pack/plugin/sql/sql-action/src/main/java/org/elasticsearch/xpack/sql/action/SqlQueryResponse.java),
  [PIT cursor lifecycle](https://github.com/elastic/elasticsearch/blob/4fe44c255c3d0da06779b921e132cdc555ed9aff/x-pack/plugin/sql/src/main/java/org/elasticsearch/xpack/sql/execution/search/SearchHitCursor.java).
- [Source pre-analysis](https://github.com/elastic/elasticsearch/blob/4fe44c255c3d0da06779b921e132cdc555ed9aff/x-pack/plugin/sql/src/main/java/org/elasticsearch/xpack/sql/session/SqlSession.java)
  and [LIKE index expansion](https://github.com/elastic/elasticsearch/blob/f2e019bd8110088070638ca779ec1543188c0f43/x-pack/plugin/ql/src/main/java/org/elasticsearch/xpack/ql/util/StringUtils.java).
- [8.19.21 shard timeout failure](https://github.com/elastic/elasticsearch/blob/4fe44c255c3d0da06779b921e132cdc555ed9aff/server/src/main/java/org/elasticsearch/search/query/SearchTimeoutException.java)
  and [9.1.10 shard timeout failure](https://github.com/elastic/elasticsearch/blob/f2e019bd8110088070638ca779ec1543188c0f43/server/src/main/java/org/elasticsearch/search/query/SearchTimeoutException.java).

## Resource and test evidence

SQL shares the existing language byte/token/depth settings with EQL: defaults
256 KiB / 10,000 tokens / depth 128, configurable ceilings 1 MiB / 100,000 / 512.
Both languages now share one 128 MiB parser reservation pool, already forecast
once per process. Each parse reserves 2 MiB plus 64 times input bytes and 4 KiB
before each token. Fresh per-call DFA caches prevent cross-request accumulation;
character/token/rule work and cancellation are checked. Parser failures release
reservations and omit query excerpts.

Thirty-iteration allocation samples on Linux amd64, Ryzen 7 5800H, Go 1.26.6:

| Grammar | Query | Input bytes | Allocated bytes/op |
| --- | --- | ---: | ---: |
| 8.19.21 | Scalar literal | 23 | 699,342 |
| 8.19.21 | Grouped aggregation | 128 | 1,601,517 |
| 8.19.21 | 1,000-value IN | 10,038 | 2,646,859 |
| 9.1.10 | Scalar literal | 23 | 699,289 |
| 9.1.10 | Grouped aggregation | 128 | 1,601,467 |
| 9.1.10 | 1,000-value IN | 10,038 | 2,646,807 |

These samples ran alongside validation jobs, so latency is not a standalone
performance claim. Allocation traffic is not peak RSS. Sustained concurrent
client/server RSS measurements remain a live release gate. An eight-second,
two-worker SQL fuzz run completed 54,666 executions for the 8.19.21 parser.
A subsequent eight-second run exercising both grammars completed 17,172 inputs
with no failure. This is bounded evidence, not exhaustive correctness proof.

Tests cover both grammars, original text/parameter precision, all source locations,
metadata LIKE expansion, catalog checks, full-text inference boundaries, async
controls, stateless row/columnar page merging, result limits, native-token rotation,
terminal pages, malformed responses, cleanup failures, batch preflight and HTTP
cancellation. MCP tests exercise SQL query/translate, mixed batches, session
ownership and SQL cleanup on disconnect. Full-repository race tests and `go vet`
passed. Generation integrity and full reproduction with the pinned JAR and a
temporary OpenJDK 17 runtime also passed; the Go binary was built successfully.

```sh
go test -race ./...
go vet ./...
go build -trimpath -o /tmp/readonly-db-mcp-es-sql ./cmd/readonly-db-mcp
python3 tools/es-language-parser/generate.py --check
python3 tools/es-language-parser/generate_sql.py --check
python3 tools/es-rest-catalog/generate.py --cache /tmp/readonly-es-rest-specs --check
```

## Live acceptance and remaining gates

`TestElasticsearchLiveSQL` is opt-in and never provisions or writes fixtures.
Reuse the two operator-seeded documents and date mapping from the
[EQL qualification record](2026-09-22-elasticsearch-eql.md), in an
`mcp-es-acceptance-*` index on an `environment: test` target. Set
`READONLY_DB_MCP_ES_CONFIG`, `READONLY_DB_MCP_ES_TARGET` and
`READONLY_DB_MCP_ES_SQL_INDEX`. The test checks native aggregate/translation,
large parameter precision, columnar pagination and complete cursor exhaustion.

No real ES target was provisioned for this increment. Remaining gates include
both pinned builds/licenses, actual advanced SQL semantic availability and
metadata permission actions, DLS/FLS, native shard timeout behavior, SQL cursor
expiry/rotation and cleanup after disconnect, proxy behavior and sustained
resource use. ES|QL source-aware parsing and execution is the next language phase.

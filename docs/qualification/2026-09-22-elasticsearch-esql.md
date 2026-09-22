# Elasticsearch native ES|QL qualification — 2026-09-22

Status: implementation, generated-parser and TLS/MCP fixture evidence. This does
**not** certify a real Elasticsearch deployment. ENRICH isolation, external
inference, experimental pragmas and persisted async profiles remain pending.

## Implemented surface and source proof

`es_query.operation=esql.query` executes the original ES|QL through synchronous
`POST /_query`. Independent batches can mix ES|QL, SQL, EQL and native DSL reads;
every member is preflighted before dispatch. Shared-PIT batches remain native
searches and do not claim cross-language snapshot consistency.

Both complete pinned grammars are compiled into distinct Go packages. The 9.1
lexer/parser imports are vendored too, byte-for-byte, with immutable source and
output hashes. ANTLR 4.13.1 remains generation-only; operators need no Java.
Release predicates are ported to Go base classes returning false for the attested
non-snapshot builds. The 8.19 EXPLAIN syntax and 9.1 FORK syntax are tested
separately, without enabling development predicates or replacing the language
with a simplified grammar. All vendored ES|QL grammars and derived parser code
retain Elastic License 2.0 notices. See
[packaging, manifest and reproduction](../../tools/es-language-parser/README.md).

Parsing/source coverage includes FROM lists, quoted comma-separated selectors,
LOOKUP JOIN, nested EXPLAIN/FORK branches, expressions, per-aggregate WHERE,
SORT, LIMIT, projection, renaming, DISSECT/GROK, MV_EXPAND, CHANGE_POINT, SAMPLE,
ROW and SHOW INFO. Function/type/plan availability remains the native server's
decision. In particular, an admitted supplied-vector KNN expression is a syntax
and no-inference proof, not a claim that every vector function is enabled in
both pinned release registries/licenses.

Every positive source must be inside the configured and requested scope. Native
aliases stay in the query. Exclusions are permitted only after a proved wildcard
within the same relation; the proof covers the entire positive superset and does
not use exclusions to expand authority. The `::data` selector uses the ordinary
local data inventory. Remote sources, failure-store selectors and date-math
resolution still need separate existing scope/authority profiles. Source
inventories are checked again before returning results, including aggregate
results that do not expose their source index in a hit envelope.

Anonymous, positional, named and typed identifier/pattern parameters and double
parameter markers retain native representation. No values are interpolated into
query text. Bound function names are inspected for full-text effects. Ordinary
MATCH operands, assignment chains, renames and mapping aliases are checked for
semantic-text inference; regular text fields remain available even when another
field is semantic. Query-string/multi-match and ambiguous pattern lineage use a
mapping-wide proof. Request filters and mapping runtime lookups use existing DSL
visitors, including their embedded source proof. Quoted literals and comments do
not become source references, mutation commands or inference commands.

## Effects, protocol and resource bounds

Synchronous request fields are taken from both pinned `RequestXContent` parsers.
The adapter forwards query, params, columnar, filter, locale, profile and
include_ccs_metadata. `format=json` and `allow_partial_results=false` are fixed.
Async retention fields are not synchronous request fields and cannot switch this
route to persisted results. Experimental `pragma`, `accept_pragma_risks` and
snapshot-only request tables await their own resource/deployment proof.

ENRICH is recognized and its policy names are collected, but it is not yet
executable in this profile. It reads historical enrichment snapshots whose
contents and access are not constrained by ordinary source-index DLS/FLS. Reading
today's policy definition does not prove prior snapshot provenance or prevent an
administrator from changing policy sources. The remaining implementation needs
an explicit, separately scoped enrichment authority/deployment profile covering
all reachable enrichment data and future changes, with live acceptance. Policy
creation/execution remains a prohibited write. COMPLETION and semantic inference
need endpoint identity, egress, credential, data-scope and cost proofs. These
pending capabilities do not classify advanced read-only syntax as mutations.

ES|QL has no pagination state in this profile. The shared request deadline closes
the native cancellable REST channel. Execution is never retried. `is_partial`
and `took` must be present; partial results, errors, failed shards, async/cursor
state or unexpected remote metadata fail whole. Unexpected execution state
invalidates target authority. Native column metadata, row/columnar values,
multivalue cells and large JSON integers are preserved without floating-point
conversion. Width/height inconsistencies and row/result/cell limits fail whole.
The adapter never appends a LIMIT; use the native LIMIT/aggregation to express
the desired result. The engine's own implicit LIMIT still has native semantics.

`drop_null_columns` remains supported. If columnar output drops every column,
its original row count is unobservable; that response is unavailable under the
row-budget contract. Retain null columns or use row output. This does not disable
ordinary columnar output or null-column dropping with observable cardinality.

ES|QL shares the existing 128 MiB parser admission pool and language limits with
SQL/EQL (defaults 256 KiB / 10,000 tokens / depth 128). Per-call DFA/prediction
caches prevent accumulated query state. Character/token access, rule entry,
source walking and lineage expansion check work/cancellation budgets. Each ES|QL
parse reserves 8 MiB plus 64 times UTF-8 input bytes, then 4 KiB before each token;
SQL/EQL retain their 2 MiB baselines. All exits release the reservation. The larger
ES|QL baseline reflects these thirty-iteration JOIN/aggregation parser samples
on Linux amd64, Ryzen 7 5800H, Go 1.26.6:

| Grammar | Allocated bytes/op | Allocations/op |
| --- | ---: | ---: |
| 8.19.21 | 5,250,503 | 72,419 |
| 9.1.10 | 4,782,597 | 65,293 |

Allocation traffic is not peak RSS. Deployment RSS, concurrent worst-case parser
loads and actual native task termination remain release gates. A ten-second,
two-worker fuzz run exercised both grammars with 20,063 inputs and no failure.

## Verification and immutable protocol sources

Tests cover both grammars, release predicates, quoted/embedded/join/branch source
scope, original parameters and number precision, parameterized full-text effects,
field lineage, implicit runtime/filter sources, forbidden effects, row/columnar
shape and limits, partial/unexpected async state, all-member batch preflight and
HTTP cancellation without replay or retained execution permits. Registry/MCP
fixtures cover ES|QL query and mixed SQL batches, and reject unproved JOIN/async
requests. An opt-in live test is included but no real ES target was provisioned.

Validation commands:

```sh
go test ./...
go test -race ./internal/dialects/elasticsearch/... ./internal/mcpserver
go vet ./...
go build -o /tmp/readonly-db-mcp-esql ./cmd/readonly-db-mcp
python3 tools/es-language-parser/generate.py --check
python3 tools/es-language-parser/generate_sql.py --check
python3 tools/es-language-parser/generate_esql.py --check
python3 tools/es-language-parser/generate_esql.py --antlr-jar /path/to/antlr-4.13.1-complete.jar --check
python3 tools/es-rest-catalog/generate.py --cache /tmp/readonly-es-rest-specs --check
```

The full generation check used the pinned ANTLR JAR and a temporary OpenJDK 17
runtime. No runtime Java dependency or extra native parser subprocess was added.

Sources inspected at the immutable profiles:

- [8.19.21 parser grammar](https://github.com/elastic/elasticsearch/blob/4fe44c255c3d0da06779b921e132cdc555ed9aff/x-pack/plugin/esql/src/main/antlr/EsqlBaseParser.g4)
  and [9.1.10 parser grammar/imports](https://github.com/elastic/elasticsearch/blob/f2e019bd8110088070638ca779ec1543188c0f43/x-pack/plugin/esql/src/main/antlr/EsqlBaseParser.g4).
- [Synchronous request parsing and parameter classifications](https://github.com/elastic/elasticsearch/blob/f2e019bd8110088070638ca779ec1543188c0f43/x-pack/plugin/esql/src/main/java/org/elasticsearch/xpack/esql/action/RequestXContent.java)
  and [token parameter binding](https://github.com/elastic/elasticsearch/blob/4fe44c255c3d0da06779b921e132cdc555ed9aff/x-pack/plugin/esql/src/main/java/org/elasticsearch/xpack/esql/parser/EsqlParser.java).
- [Source/selector handling](https://github.com/elastic/elasticsearch/blob/f2e019bd8110088070638ca779ec1543188c0f43/x-pack/plugin/esql/src/main/java/org/elasticsearch/xpack/esql/parser/IdentifierBuilder.java)
  and [JOIN, ENRICH and branch plan building](https://github.com/elastic/elasticsearch/blob/f2e019bd8110088070638ca779ec1543188c0f43/x-pack/plugin/esql/src/main/java/org/elasticsearch/xpack/esql/parser/LogicalPlanBuilder.java).
- [Cancellable synchronous REST handler](https://github.com/elastic/elasticsearch/blob/f2e019bd8110088070638ca779ec1543188c0f43/x-pack/plugin/esql/src/main/java/org/elasticsearch/xpack/esql/action/RestEsqlQueryAction.java),
  [response encoding](https://github.com/elastic/elasticsearch/blob/f2e019bd8110088070638ca779ec1543188c0f43/x-pack/plugin/esql/src/main/java/org/elasticsearch/xpack/esql/action/EsqlQueryResponse.java)
  and [cross-cluster metadata conditions](https://github.com/elastic/elasticsearch/blob/f2e019bd8110088070638ca779ec1543188c0f43/x-pack/plugin/esql/src/main/java/org/elasticsearch/xpack/esql/action/EsqlExecutionInfo.java).
- [Semantic-text MATCH behavior](https://github.com/elastic/elasticsearch/blob/f2e019bd8110088070638ca779ec1543188c0f43/x-pack/plugin/esql/src/main/java/org/elasticsearch/xpack/esql/expression/function/fulltext/Match.java),
  [supplied-vector KNN](https://github.com/elastic/elasticsearch/blob/f2e019bd8110088070638ca779ec1543188c0f43/x-pack/plugin/esql/src/main/java/org/elasticsearch/xpack/esql/expression/function/vector/Knn.java)
  and [ENRICH snapshot resolution](https://github.com/elastic/elasticsearch/blob/f2e019bd8110088070638ca779ec1543188c0f43/x-pack/plugin/esql/src/main/java/org/elasticsearch/xpack/esql/enrich/EnrichPolicyResolver.java).

## Live acceptance and remaining work

`TestElasticsearchLiveESQL` never provisions or writes fixtures. Reuse the
operator-seeded dated documents from the [EQL record](2026-09-22-elasticsearch-eql.md)
on an `environment: test` target and an `mcp-es-acceptance-*` index. Set
`READONLY_DB_MCP_ES_CONFIG`, `READONLY_DB_MCP_ES_TARGET` and
`READONLY_DB_MCP_ES_ESQL_INDEX`. The test exercises native aggregation, date sort,
projection/LIMIT and a parameter larger than JavaScript's exact integer range.

Both pinned builds still require live acceptance for native syntax/function
availability, LOOKUP JOIN index mode and licensed features, FORK behavior,
DLS/FLS, mapped runtime fields, completion metadata, task cancellation through
proxies and sustained client/server resource bounds. ENRICH isolation and its
future-policy/snapshot authority remain explicit implementation work, alongside
inference, cross-cluster, API-key/attestor and persisted async profiles in RFC-0006.

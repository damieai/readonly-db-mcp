# Elasticsearch synchronous EQL qualification — 2026-09-22

Status: implementation, generated-parser and TLS/MCP fixture evidence. This
record does **not** certify real Elasticsearch deployments. SQL, ES|QL, SQL
cursors and persisted async EQL remain separate pending profiles.

## Implemented surface

`es_query.operation=eql.search` runs native synchronous EQL against structured
`indices`. Independent batches can mix EQL and other native reads; all members
are preflighted before execution under one deadline and encoded result budget.
EQL is rejected as an incompatible shared-PIT batch member or cursor kind.

The complete official `EqlBase.g4` is identical in the pinned 8.19.21 and 9.1.10
commits. ANTLR 4.13.1 generates the checked-in Go lexer/parser/listener files;
the pinned Go runtime is also 4.13.1. No Java process or external parser binary
is required at runtime. The generator verifies grammar, generator and generated
file hashes and supports offline integrity checks and full reproduction.
The upstream grammar and generated files retain Elastic License 2.0; see
[packaging, notices and regeneration](../../tools/es-language-parser/README.md).

The parser consumes one statement through EOF and identifies event, sequence,
sample or join syntax, including expressions, functions, time windows, missing
events, optional fields, comments, pipes, join keys and termination conditions.
The adapter sends the original admitted text unchanged. Grammar acceptance does
not claim the native engine implements every semantic construct: native type,
function, pipe and join support is still enforced by that ES version.

The grammar's data source is the enclosing request's index scope. Event-category
names, strings and identifiers are never treated as source selectors or mutation
keywords. Native `filter` references, wrapper queries, runtime lookups and stored
script definitions are handled by the existing scoped DSL visitors. The complete
request body fields declared by the two pinned native request parsers are covered;
modern documentation for other versions does not automatically add capabilities.

## Effect and response boundaries

The pinned native execution code selects async work when
`wait_for_completion_timeout` is present and non-negative. This profile accepts
omission or exactly `"-1"`, and does not translate MCP timeout into that parameter.
The REST channel is cancellable. `keep_on_completion: true` is rejected; partial
search/sequence results are disabled explicitly instead of inheriting cluster
defaults. `keep_alive` alone does not select the async branch.

Native responses retain event and sequence arrays, join keys, `_source`, requested
fields and exact integers. `missing: true` only exempts the native empty
index/ID/source placeholder, never a payload from another index. `max_rows`
bounds total events (including missing placeholders) and sequence count. Native
`size` keeps its event/group-count semantics; oversized results fail whole and
no sequence is cut or silently reduced.

Timeout, partial and shard-failure responses return no data. A persisted-result
ID or running response from a supposedly synchronous operation is a profile
mismatch: the target becomes unhealthy, and neither the ID nor native error text
is returned. Creation/advancement of unknown async work is never retried or cleared
using a caller-supplied identifier.

Protocol evidence was checked in both pinned versions:

- [8.19.21 request fields](https://github.com/elastic/elasticsearch/blob/4fe44c255c3d0da06779b921e132cdc555ed9aff/x-pack/plugin/eql/src/main/java/org/elasticsearch/xpack/eql/action/EqlSearchRequest.java)
  and [9.1.10 request fields](https://github.com/elastic/elasticsearch/blob/f2e019bd8110088070638ca779ec1543188c0f43/x-pack/plugin/eql/src/main/java/org/elasticsearch/xpack/eql/action/EqlSearchRequest.java).
- [8.19.21 synchronous/async dispatch](https://github.com/elastic/elasticsearch/blob/4fe44c255c3d0da06779b921e132cdc555ed9aff/x-pack/plugin/eql/src/main/java/org/elasticsearch/xpack/eql/plugin/TransportEqlSearchAction.java)
  and [9.1.10 dispatch](https://github.com/elastic/elasticsearch/blob/f2e019bd8110088070638ca779ec1543188c0f43/x-pack/plugin/eql/src/main/java/org/elasticsearch/xpack/eql/plugin/TransportEqlSearchAction.java).
- [Cancellable REST handler](https://github.com/elastic/elasticsearch/blob/4fe44c255c3d0da06779b921e132cdc555ed9aff/x-pack/plugin/eql/src/main/java/org/elasticsearch/xpack/eql/plugin/RestEqlSearchAction.java)
  and [native event/sequence encoding](https://github.com/elastic/elasticsearch/blob/4fe44c255c3d0da06779b921e132cdc555ed9aff/x-pack/plugin/eql/src/main/java/org/elasticsearch/xpack/eql/action/EqlSearchResponse.java).

## Resource evidence

| Control | Default | Configurable ceiling |
| --- | --- | --- |
| Language bytes | 256 KiB | 1 MiB; enclosing request byte limit also applies |
| Tokens, including hidden tokens and EOF | 10,000 | 100,000 |
| Expression delimiter/prefix depth | 128 | 512; native version limits also apply |
| Native EQL fetch size ceiling | 10,000 | 100,000; native default remains 1,000 |
| Parser process pool | 128 MiB | Fixed |

Per-call reservation is `2 MiB + 64 * query UTF-8 bytes`, plus 4 KiB reserved
before each token is fetched. Lexer/parser prediction caches are per call;
request data cannot accumulate in a process-global DFA. Character/token work,
parse rule entry, tokenized recursion depth and request cancellation are checked.
All exits release the reservation; no parser goroutine continues after timeout.
The global pool is included once in configuration forecasting.

Allocation sampling on Linux amd64, AMD Ryzen 7 5800H, Go 1.26 toolchain,
30 iterations per benchmark after grammar initialization:

| Query | UTF-8 bytes | Time/op | Allocated bytes/op |
| --- | ---: | ---: | ---: |
| Event predicate | 14 | 0.538 ms | 347,355 |
| Two-stage sequence | 118 | 1.617 ms | 1,140,829 |
| 1,000-value IN list | 10,016 | 5.026 ms | 2,963,776 |

These measure allocation traffic, not peak RSS or a universal worst-case bound.
Small queries have substantial fixed parser overhead; admission therefore includes
a fixed reserve, not just a text multiplier. Production concurrency/RSS and larger
adversarial allocation measurements remain release gates.

## Tests and reproduction

- Grammar tests cover both versions, complex sequences/samples/joins, functions,
  missing events, quoted identifiers, nested comments, mutation-looking literals,
  multiple statements, malformed input, limits, cancellation and concurrent calls.
- TLS tests exercise both pins, unmodified native text/options, runtime script
  number precision, embedded source denial, async/body-option controls, complete
  sequence results, forged missing markers, shard failures and profile drift.
- MCP tests include EQL reads, mixed batches, large integers and hostile envelopes.
  Configuration tests cover defaults and all added ceilings. HTTP cancellation
  reaches the fixture and releases query admission without a retry.
- An 8-second two-worker fuzz run completed 43,550 executions with no failure.
  It is bounded evidence, not proof of parser correctness for all inputs.

```sh
go test -race ./...
go vet ./...
go build -trimpath -o /tmp/readonly-db-mcp-es-eql ./cmd/readonly-db-mcp
python3 tools/es-language-parser/generate.py --check
python3 tools/es-rest-catalog/generate.py --cache /tmp/readonly-es-rest-specs --check
```

Full generator reproduction was checked with the pinned JAR and a temporary
OpenJDK 17 runtime. No system runtime installation is part of the deliverable.
Reproduction includes deterministic removal of unreachable ANTLR Go label
markers and newly unused labels; the complete grammar remains unchanged.

## Live acceptance setup and remaining gates

`TestElasticsearchLiveEQL` is opt-in and performs no fixture writes. Provision an
`environment: test` target and an index named `mcp-es-acceptance-*` within the
allowed scope. Map `@timestamp` as date and `event.category`, `event.type`,
`host.id` as keyword, then seed and refresh these two operator-owned documents:

```json
{"@timestamp":"2026-09-22T00:00:00Z","event":{"category":"process","type":"start"},"host":{"id":"qualification"}}
{"@timestamp":"2026-09-22T00:00:01Z","event":{"category":"network","type":"connection"},"host":{"id":"qualification"}}
```

Run with `READONLY_DB_MCP_ES_CONFIG`, `READONLY_DB_MCP_ES_TARGET` and
`READONLY_DB_MCP_ES_EQL_INDEX` set. The live test requires nonempty event,
sequence, missing-event and sample results. Pair it with the existing native
write-denial probe and lifecycle tests from earlier qualification records.

No real ES target was provisioned for this increment; the live test is skipped.
Remaining gates include both native builds/licenses, actual function/pipe support,
permission actions, DLS/FLS, cancellation and internal context cleanup after
disconnect, proxy behavior, large sequence resource use, and server/client RSS
under sustained concurrent workloads. SQL and ES|QL need their own source-aware
grammars and execution profiles next.

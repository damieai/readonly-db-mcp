# Architecture

```text
MCP client over stdio
        |
        v
typed MCP tool schemas
        |
        v
exact target registry lookup
        |
        v
engine-specific SQL policy or Redis command policy
        |
        v
bounded fair admission by workload class
        |
        +----> target-local metadata/result caches
        |
        v
native read-only transaction or SQL Server SHOWPLAN proof / attested Redis command
        |
        v
dedicated SELECT-only SQL identity / read-key-only Redis ACL
```

## Packages

- `cmd/readonly-db-mcp`: flags, signal handling and process lifecycle.
- `internal/mcpserver`: MCP input/output contracts; no SQL implementation.
- `internal/registry`: exact-alias target lookup and lifecycle ownership.
- `internal/core`: engine-neutral target and result contracts.
- `internal/dialects/mysql`: MySQL connection, grant attestation, AST policy,
  metadata reads and bounded result collection.
- `internal/dialects/postgresql`: PostgreSQL connection, role/object privilege
  attestation, native AST policy and metadata reads.
- `internal/dialects/sqlserver`: SQL Server connection, effective-permission
  attestation, pinned ScriptDom process, transitive module/catalog proof, mandatory
  `SHOWPLAN_XML` proof, snapshot batches, and catalog metadata reads. An optional
  read-only attestor uses one connection reserved from the target capacity.
- `tools/sqlserver-parser`: self-contained ScriptDom helper and hostile/advanced
  grammar corpus integration. Parsing runs under admission and the request deadline;
  its separate process memory must be included in deployment capacity.
- `internal/dialects/redis`: Redis ACL and live command-catalog attestation,
  signed module profiles, canonical Search index/prefix verification, Sentinel and
  Cluster route attestation, RESP normalization and bounded command execution.
- `internal/dialects/elasticsearch`: pinned metadata routes, effective-user
  authority proofs, bounded glob containment/resolution, strict native JSON,
  HTTP/1.1 TLS/socket leases and periodic re-attestation. Phase 1 exposes only
  resolve/mappings; advanced query/language/lifecycle handlers remain pending.
- `tools/es-rest-catalog`: reproducible REST catalog extraction from immutable
  upstream commits, with reviewed endpoint effects and source hashes.
- `internal/config`: strict YAML decoding, hard ceilings and secret resolution.
- `internal/audit`: structured, non-content audit events.
- `internal/admission`: global/per-target bounded queues, workload fairness and
  cancel-safe permits.
- `internal/metrics`: content-free counters, phase durations and periodic stderr
  summaries.

Each MySQL target owns immutable normalized policy indexes and bounded caches.
Metadata, interactive, batch and maintenance work share one admission controller;
no runtime database operation bypasses it. Final query and batch structures are
JSON-sized before return, and batch execution stops when another minimal result
cannot fit the remaining response budget.

## Agent workload latency budget

This server is an infrastructure tool for agents, so its defaults must tolerate
multi-step investigations and genuinely slow analytical reads without making
short deadlines an accidental availability boundary. Agent reasoning between
tool calls does not hold an admission permit, transaction, or database
connection. Only an active MCP tool call may lease those resources.

The common operational profile is a cross-engine compatibility contract:

| Control | Baseline |
| --- | ---: |
| Default MCP query or batch deadline | 5 minutes |
| Configured maximum request deadline | 10 minutes |
| Compiled request hard ceiling | 15 minutes |
| Default admission queue wait | 1 minute |
| Compiled queue-wait hard ceiling | 5 minutes |
| Global / per-target concurrency | 16 / 8 |
| Batch / maintenance concurrency | 4 / 2 |
| Open / idle connections per target | 8 / 4 |
| Connect / write timeout | 15 seconds |
| Read timeout | maximum request deadline plus 1 minute |
| Connection lifetime / idle time | 30 / 10 minutes |

`internal/config` and `configs/example.yaml` are the executable sources of truth
for these values. Tests must keep them aligned. An engine RFC inherits this
profile unless it documents a measured infrastructure constraint and retains an
operator-configurable path to the common request budget.

A request deadline starts before admission and covers queueing, policy or plan
preflight, transaction setup, execution, result collection, and cleanup. Batch
deadlines cover the entire batch unless the public contract explicitly says
otherwise. Driver socket read deadlines must exceed the maximum request
deadline. Shorter connect, write, lock, discovery, or cleanup deadlines are
permitted when they fail one phase quickly rather than reducing the execution
budget.

New engines and driver upgrades must not introduce an implicit timeout below
the remaining request deadline. Pool settings must map to actual driver hard
ceilings, not merely preferred or minimum pool sizes. Cancellation tests must
show that server work stops and the connection or permit becomes reusable.

Tightening the common profile is a compatibility and availability change. It
requires an explicit RFC, benchmark or production evidence, migration notes,
and tests for long-running requests, saturation, cancellation, and pool
recovery. Operators may always choose stricter values for a particular target.

## Adding a database engine

A new engine gets its own `internal/dialects/<engine>` package. It implements the
minimal `core.Target` plus `core.SQLTarget` or `core.RedisTarget`, and an
appropriate batch capability. It must not reuse another engine's parser or
privilege assumptions.

An engine is considered complete only when it has:

1. A maintained dialect parser or live command capability catalog and a
   fail-closed side-effect policy.
2. Effective privilege attestation for the connected identity.
3. A database-native read-only transaction or read-only ACL/key enforcement.
4. Safe engine-specific metadata/introspection paths.
5. Cancellation and resource-limit integration tests.
6. A hostile query/command corpus demonstrating that public tools cannot mutate
   a disposable database.
7. A latency and pool budget conforming to the common agent workload profile.

## Why target selection is stateless

Every tool call contains `target`. A mutable `USE DATABASE` tool would make
concurrent requests and multiple clients capable of changing each other's
destination. Exact per-call target selection also keeps audit records complete
and makes a response self-describing.

## Why arbitrary DSNs are forbidden

Allowing a model to supply a host or DSN would turn the MCP server into an SSRF
and credential-routing primitive. Network destinations and credentials are
operator configuration, never tool input.

## Elasticsearch metadata and native query resource accounting

Metadata calls start the shared deadline before admission. Queued index selectors
use a process-wide 64 MiB byte budget; admitted adapter work reserves from a
separate 512 MiB budget before transport or proof parsing. Each reservation is
`6 * max_result_bytes + 64 * max_json_nodes + max_request_bytes + 2 MiB`.
The extra proof space bounds glob automata; their own retained-state ceiling is
1 MiB with separate state/transition caps. Configuration forecasts include ES
JSON/proof overhead and keep the existing aggregate 1 GiB forecast ceiling.
These are conservative adapter reservations, not a measured process RSS promise;
MCP delivery backlog and live-server saturation measurements remain release gates.

One shared transport enforces total open/idle sockets across explicit endpoints.
It retires expired idle sockets and lets active requests finish before retirement.
Connect/TLS and writes have their own phase budgets; the request context bounds
execution and collection. No runtime credential can be selected by a caller.
Metadata responses fail on oversized native JSON or encoded MCP envelopes, with
no silent truncation. Target-level metadata caches are not used for ES proofs.

Native query/batch calls reuse these budgets and reserve an additional six times
their serialized request bytes for decoded DSL, rendered templates, frozen
scripts and batch framing. Configuration forecasts include up to seven request
copies. One lease covers the whole request: all source proofs and batch members
share one deadline, one admission permit and one combined encoded response cap.
Retained mapping/script proof bytes are bounded together; rendered member bodies
are bounded together. Whole-envelope JSON node/depth limits cover batch input.

Effect dispatch uses the pinned REST catalog. Contextual native DSL visitors
preserve literals and script parameters while resolving executable source
positions. Mtermvectors defaults are materialized in original input order before
source validation. Compatible search batches use generated NDJSON and limit
native concurrent searches to one; heterogeneous batches preserve each operation
sequentially. Those modes report independent consistency. PIT batches instead
preflight compatible searches, open one owned PIT, apply rotating IDs to every
member, and close it before returning `consistency: pit`. Partial shard/time
results fail, and requested aggregate semantics are never shortened to fit limits.


Elasticsearch contexts have a separate process registry budget: 128 slots and
64 MiB retained state, included once in the configuration memory forecast.
Each target defaults to 32 contexts (maximum 128). Reservations precede native
creation and include frozen DSL, snapshot mappings/source inventories, native IDs
and continuation sort values. Per-handle gates serialize fetch and close without
holding the registry mutex during network or admission waits. The MCP server
assigns a nonce to the actual transport session; SDK session IDs may be empty on
stdio. Disconnect cancels that owner's requests and initiates owned cleanup.

PIT stores the resolved snapshot and mapping proof, permitting alias rollover
without substituting a new reader. Subsequent PIT searches use no index URL,
routing or preference; those are applied at creation. Live embedded sources,
including mapping runtime lookups, are proved on each page. Explicit PIT bodies
support aggregation continuation and advanced DSL. Scroll preserves its frozen
query, page size and slices. Both paths remove internal context IDs while
preserving native JSON numbers. No raw native continuation/clear operations are
available through `es_query`.

Observed privilege-proof changes (including DLS/FLS), stale proof and closed
sessions invalidate handles. Normal cleanup uses the remaining caller deadline;
a canceled request schedules a separate five-second maintenance attempt and
returns without waiting for that recovery. Background cleanup reserves response
memory and maintenance admission. Unconfirmed creation/rotation/cleanup keeps
both slot and retained memory until the conservative native lease horizon.
Native lease accounting retains the longest requested keep-alive, since shorter
renewals cannot be assumed to shorten ES reader retention. Local idle and
absolute deadlines still bound handle usability. The horizon for an ambiguous
request includes the maximum request interval. Actual task cancellation and
native context lifetime measurements remain live-server qualification gates.


Synchronous Elasticsearch EQL is an `es_query` operation (`eql.search`) and an
independent batch member. It uses a generated Go ANTLR parser from identical
upstream grammars at both pinned commits. `singleStatement` must reach EOF; the
structured analysis identifies event/sequence/sample/join syntax and the
`request_indices` source contract. EQL text is sent unchanged to the fixed
scoped `/{indices}/_eql/search` route. Mapping/runtime and native filter proofs
reuse contextual DSL visitors. Native semantic validation determines supported
functions/pipes/joins; the adapter does not substitute another query language.

Parser state has a distinct process-wide 128 MiB semaphore included in forecasts.
Each call reserves `2 MiB + 64 * language_bytes`, then 4 KiB before each token.
Per-invocation DFA/prediction caches prevent unbounded cross-request retention.
Character/token operations and rule entry check cancellation/work budgets;
tokenized prefix/delimiter depth and rule depth bound recursion. Language text,
tokens and depth have explicit operator limits. Errors never print lexer input.
Java and ANTLR's JAR are only maintainer generation tools; runtime is Go-only.
Upstream and generated grammar files retain their own license notices and hashes.

The pinned EQL native transport selects async execution when its wait timeout
is present and non-negative. The adapter admits omission or the exact `-1`
sentinel, rejects result retention and requires complete results. It does not
map the MCP deadline into this async control: HTTP cancellation uses the native
cancellable REST channel. EQL responses retain event/sequence grouping, join
keys and empty missing-event placeholders. Placeholder markers cannot exempt
real data from index validation. Aggregate event and sequence counts share
`max_rows`, with whole-result failure on overflow. An unexpected async result ID
or running response invalidates the target profile instead of being exposed.
Real-server task cancellation, temporary context cleanup and allocation/RSS
saturation still require deployment qualification.

Native SQL is exposed as `sql.query`, `sql.translate` and the `sql` cursor kind.
Two complete version-specific upstream grammars generate Go parsers; a bounded
case-changing character stream changes lexer lookahead only, preserving original
identifiers, literals and query text. Tree visitors collect all relation nodes,
CTE definitions/references, nested branches, metadata LIKE patterns/parameters,
catalogs and full-text field selectors. SQL LIKE `_` is conservatively expanded
to `*`, matching the native mapping resolver rather than assuming SQL row-filter
semantics for metadata enumeration. The SQL pre-analyzer resolves relation names
before CTE substitution, so references are included in source proof too.

SQL request fields come from both pinned native request parsers. Filters/runtime
mappings use existing DSL visitors; MATCH/QUERY additionally check semantic-text
inference authority. Statements with no source need no index resolution. Local
catalog identities are read from the pinned cluster, while remote catalog
execution remains a separate profile. The native server decides which admitted
SQL functions/plans its build can execute. Translation returns its native DSL.

Synchronous SQL can create transient pagination state. It reserves an owned
context before dispatch, captures rotated tokens before validating output, and
uses SQL-specific `POST /_sql/close` cleanup. Stateless execution drains pages
within a single deadline and total row/byte limit; session cursors instead expose
one page per call. Continuation retains only output controls and scope proof;
it never replays the original SQL or accepts a raw token. Terminal pages close
on absent/empty native cursor even when rows remain. Row and columnar dimensions
are validated, values retain exact JSON numbers, and oversized results fail whole.
Cleanup, uncertainty, authority changes and session disconnection reuse the shared
owned-context machinery. SQL and EQL have fresh per-call parser DFA caches and
share one 128 MiB reservation pool already included in resource forecasting.

Synchronous Elasticsearch ES|QL is `esql.query`, available in individual queries
and independent batches. Each pinned version has its complete upstream lexer,
parser and grammar imports compiled to Go. Generation preserves native release
predicates with small Go base classes; development-only commands stay unavailable
on the attested release builds. Structured source visits cover every relation,
including JOIN targets and nested EXPLAIN/FORK branches. Quoted source lists are
split according to native source semantics. Exclusions are preserved in the
query while proof covers the positive superset. The default `::data` selector
uses the existing data inventory; remote/failure-store authority remains separate.

Parameter binding mirrors native anonymous, positional, named and identifier
forms. It never expands query text. Full-text function/field parameters and
assignment/rename lineage are inspected for implicit semantic inference. Mapping
runtime references and request filters reuse existing DSL source/effect visitors.
The native engine receives the original query and parameters and still validates
function/type/plan availability. Source-free ROW/SHOW queries need no index proof.
ENRICH has a separate optional data namespace: `enrich.scope=all_cluster_snapshots`
grants every historical/current/future snapshot in the pinned local cluster.
This grant cannot be narrowed by the normal index allow/deny lists or source
DLS/FLS; deployments needing those boundaries must provision a separate cluster.
The runtime user must have explicit read-only `monitor_enrich`. Snapshot inventory
uses internal fixed GET cluster-state and policy routes, never caller URLs.
Stock snapshot mapping/write-block/alias checks include retired indices. Active
policy type/match field are compared with the native snapshot; policy build
indices/query remain historical metadata and are not executed as query filters.
The inventory and selected policy digests are checked before/after execution,
including whole batches; observed drift fails without replay. State comparisons
are consistency checks, not distributed locks or a confidentiality proof. The
explicit all-snapshot grant supplies that data scope. Native snapshot provisioning
and stock node/plugin profiles remain deployment requirements.

ENRICH adds no cursor or persistent state. Inventory bytes and combined physical
source counts share existing proof budgets; cancellation covers metadata reads.
Startup/periodic attestation includes the inventory digest, so changes invalidate
owned contexts. Internal proof routes are not public query/metadata operations;
policy creation, execution and deletion are classified as writes.

ES|QL routes only to synchronous `POST /_query`, with JSON format and complete
results. Locale, profiling, columnar output and null-column dropping remain native
controls. Async retention controls, caller transport options and experimental
resource pragmas are not part of this profile. Returned rows/columnar heights
and widths are validated without flattening multivalue cells. `max_rows` bounds
the whole result and fails on overflow; queries are never rewritten with an
implicit adapter LIMIT. Fully dropped columnar output cannot prove row count
and fails with an instruction to retain columns or use row output. Unexpected
async/cursor or cross-cluster state invalidates the profile. Source inventories
are rechecked before data is released. Cancellation closes the native cancellable
REST channel; no ES|QL pagination state is retained locally.

ES|QL shares the 128 MiB parser admission pool with SQL/EQL, reserving an 8 MiB
baseline plus the same input-byte and per-token charges. The larger baseline is
based on allocation samples of the richer mode-based grammar; actual peak RSS
and native task termination remain live qualification gates. All grammars retain
per-invocation prediction caches, request cancellation and explicit syntax/work
limits. Runtime remains Go-only; Java is solely a maintainer generation tool.

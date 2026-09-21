# RFC-0007: Dense vector retrieval for Redis and PostgreSQL

- Status: P0 Redis implemented; P1 catalog/binding foundation implemented, admission gates open; P2–P3 pending
- Created: 2026-09-21
- Depends on: RFC-0001, RFC-0002, RFC-0003, RFC-0004
- Scope: Redis Search vector indexes and PostgreSQL pgvector

## Summary

Complete dense retrieval through the existing native query tools. Redis already
has a binary-safe Search command path but lacks real vector qualification.
PostgreSQL needs extension-aware authority and expression proofs before pgvector
can be used. Installing an extension alone is not sufficient in either engine.
Keep advanced read-only queries, filters, joins, aggregations and tuning options;
enforce persistent-effect, object-scope and resource boundaries.

## Context and current evidence

Update: P0 now has a [native qualification record](qualification/2026-09-21-redis-dense-vectors.md)
and [usage guide](REDIS-VECTOR-RETRIEVAL.md). P1 has an internal
[catalog/native binding foundation](qualification/2026-09-21-postgresql-vector-proof.md)
with PostgreSQL fixture evidence. Pre-analysis callback admission and complete
dependency closure remain open; no production pgvector exception is enabled.
The following table records the pre-implementation assessment that motivated
the plan.

| Engine | Current implementation | Missing evidence or behavior |
| --- | --- | --- |
| Redis Search | `redis_command` preserves Search arguments, decodes base64 blobs, requires signed module profiles and checks live index prefixes | Real dense KNN/range/filtered retrieval, binary payload round trip, scores and index-specific acceptance |
| PostgreSQL pgvector | Native SQL parser and `$n` parameters exist; normal queries use read-only transactions | Startup rejects executable non-system functions; no extension identity/type/operator/cast proof, vector codec qualification or request-local search tuning |

Repository evidence:

- [Redis argument decoding](../internal/dialects/redis/policy.go) accepts string
  or base64 arguments. Search prefix validation preserves query contents.
- [Search policy tests](../internal/dialects/redis/search_indexes_test.go) contain
  a filtered KNN expression, but its vector argument is placeholder text.
- [The local Search fixture](../internal/dialects/redis/local_integration_test.go)
  creates TAG/NUMERIC fields and executes an aggregation, not a vector query.
- [PostgreSQL identity checks](../internal/dialects/postgresql/target.go) reject
  any executable non-system function; `search_path` is pinned to `pg_catalog`.
- [PostgreSQL policy](../internal/dialects/postgresql/policy.go) approves functions
  by built-in name and rejects non-system qualification. It does not bind
  operator/cast identities. A vector SQL fragment passing this syntax check is
  not proof of supported execution or safe extension admission.

No live vector database was exercised in this assessment. Earlier Search
aggregation/topology passes are not dense-retrieval acceptance.

## Goals

- Return native document/row IDs, distances and requested fields using supplied
  dense vectors, with exact search and supported approximate indexes.
- Preserve filtering, ranges, native expressions, aggregation, SQL CTEs, joins,
  window functions and reranking compositions within proven scope.
- Publish per-version capabilities and separate implemented from qualified.
- Demonstrate application rejection and native denial of persistent writes.

## Non-goals

Installing extensions, creating/training indexes, loading embeddings or calling
embedding providers through MCP are outside this retrieval task. Operators own
provisioning. Redis vector-set commands and unrelated vector plugins need their
own capability profiles; Search support must not imply support for all plugins.
Sparse and binary retrieval are not release gates for this dense milestone;
compatible read-only native expressions must not be banned merely by category.

## Proposed design

### Redis: retain the existing native command contract

Use `redis_command` / `redis_batch`, `FT.SEARCH` and other explicitly profiled
read-only Search operations. Supply vector blobs through `{ "base64": "..." }`
inside `PARAMS`; validate decoded bytes against the existing argument budget.
Document the dtype, dimensions and byte-order contract of each tested profile.
Do not tokenize or rewrite Search query expressions to a small KNN grammar.

Add vector-field capability inspection from `FT.INFO`: storage kind, algorithm,
dimensions, dtype and metric. Retain live canonical-index/prefix checks, signed
artifact/build identity, scoped ACLs and proof refresh. Metadata helps clients
construct valid requests; a native dimension/type error is not a mutation error.
Known unsupported version features must produce a capability error.

Qualify HASH and JSON indexes where the selected build supports them, FLAT and
HNSW, L2/IP/COSINE, KNN, vector range and filters. Preserve native search tuning,
projection, sorting and dialect options. New algorithms/dtypes are added through
versioned profiles and tests, not rejected because they are advanced. The native
syntax and metrics are described in [Redis vector documentation](https://redis.io/docs/latest/develop/ai/search-and-query/vectors/).

### PostgreSQL: extension capabilities, not a name whitelist

Add an opt-in pgvector profile under PostgreSQL target configuration, binding
the supported server version, extension version, installation schema and reviewed
object signatures. Default-off retains current deployments' behavior. Discover
live OIDs from `pg_extension` and extension membership through `pg_depend`;
compare types, procedures, operators, casts, aggregates, operator classes and
access-method support functions against the reviewed profile. OIDs are local to
one database and cannot be hardcoded into a release manifest.

Replace the blanket non-system-function rejection with exact approved extension
capabilities while continuing to reject unexplained effective EXECUTE grants.
Include inherited PUBLIC grants in the inventory. Never admit every function
in an extension schema, or trust an IMMUTABLE/STABLE label alone. Native library
identity is an operator/deployment trust assertion; catalog metadata cannot
cryptographically prove a remote shared library. Keep untrusted schema CREATE,
ownership and security-definer escalation unavailable to the runtime role.

Resolve each expression's actual function/operator/type/cast signatures, including
overloads, implicit coercions, aggregate support functions and type I/O. Extend
the current syntax walker and catalog proof together. Pin a safe search path
containing only system and verified extension namespaces; support both idiomatic
vector SQL and explicit schema-qualified types/operators. Do not install the
extension into `pg_catalog` to bypass the existing check. PostgreSQL's
[operator resolution rules](https://www.postgresql.org/docs/current/typeconv-oper.html)
make namespace, input types and overloads part of the proof.

P1 below must settle a binding implementation with differential native-server
tests before any extension EXECUTE grant is admitted. Raw parse trees, procedure
names and ordinary EXPLAIN output alone are insufficient binding evidence.
If a Go resolver cannot prove the full required expression set, implement a
reviewed native analysis helper; do not silently drop joins/CTEs/casts from scope.
Planning itself may invoke extension callbacks, so dependency admission must
precede any planner-based inspection. Include dependencies reached through
views, RLS, expressions and chosen indexes, plus invalidation on catalog drift.

Reuse `query_select`, `query_batch` and `query_explain`, with the same internal
validation path. Text vector parameters with explicit trusted type casts provide
a compatible initial binding; do not claim JSON-array parameters work today.
Add tested bounded decoding for vector/halfvec values and preserve native
distance semantics. Do not convert SQL to a generic top-K-only interface.

Propose a typed optional `postgresql_options` request object for reviewed
transaction-local retrieval controls. Validate version, type, range and memory
effects, then apply internally with bound values inside the read-only transaction.
Include HNSW/IVFFlat search breadth and iterative-scan controls where supported.
Settings must reset on success, error, cancellation and pool reuse; a batch has
one declared options scope. Arbitrary SET, role/search-path changes and raw
`set_config` calls remain unavailable. This preserves useful local tuning
without granting persistent or cross-request changes.

The intended dense baseline is pgvector `vector` and `halfvec`, exact retrieval,
HNSW and IVFFlat where each metric/type combination is supported, L2/cosine/inner
product, plus L1 where available. Filtered iterative scans and SQL reranking are
part of the planned acceptance. See [pgvector's native query and index contract](https://github.com/pgvector/pgvector).

## Security and trust boundaries

Runtime identities cannot install/drop extensions or indexes, insert/update/delete
vectors, mutate ACLs, write files or invoke external services. Provision fixtures
with a separate administrator; never pass its credentials to MCP. Database
read-only mode is one layer, not proof that arbitrary native code has no effects.

Reject forged/replaced extension members, unknown executable dependencies, unsafe
search-path shadowing, scope escapes and expired identity proof. Test same-name
overloads and casts, malicious type I/O, domains, view/RLS dependencies and
concurrent extension/index changes. Complete proof must be bound to the execution
identity and refreshed before stale statements can run. Administrative concurrent
DDL remains an explicit trusted-operator boundary; do not claim a periodic check
eliminates every race.

Redis alias/prefix drift and native module ACL enforcement remain required,
including where module commands do not behave like ordinary key commands.
Native denial tests must target disposable owned fixtures. Database/function
privilege semantics follow the [PostgreSQL GRANT contract](https://www.postgresql.org/docs/current/sql-grant.html).

## Latency, admission, and pool budget

Inherit [the common workload contract](ARCHITECTURE.md#agent-workload-latency-budget).
No new vector-specific short deadline or pool is introduced.

| Boundary | Default | Maximum / ceiling | Coverage and recovery |
| --- | --- | --- | --- |
| Query or whole batch | 5 minutes | Configured 10 minutes; hard 15 minutes | Admission, proof, bind, execute, collect and cleanup; no per-item renewal |
| Admission queue | 1 minute | 5 minutes and remaining request time | Existing global/target/operation permits |
| Connection acquisition | Remaining request time | Request deadline | Existing target pool; no hidden helper pool |
| Connect/discovery/write | Existing engine configuration; common connect/write 15 seconds | Existing validated phase bounds and remaining context | Fail that phase; do not shorten long query execution |
| Driver read | Request maximum plus 1 minute | Existing validated configuration | Request cancellation remains authoritative |
| Server execution/lock | Engine profile and remaining deadline | No renewal past request deadline | PostgreSQL cancellation; Redis native timeout behavior measured per build |
| Cleanup/drain | Existing bounded engine cleanup | Connection discarded if safe reset cannot be proven | Return permits; do not return a dirty session to the pool |

Reasoning between MCP calls holds no lease. Helpers/catalog calls consume the
same target-wide connection and admission budgets. Verify configured hard pool
ceilings rather than relying on driver preferences. Maintenance and metadata use
existing separate quotas. Redis client cancellation does not automatically prove
server computation stopped; native timeout/cancellation tests are a release gate.

## Resource and response budget

Retain global/target concurrency 16/8, batch/maintenance 4/2 and target open/idle
connections 8/4 by default. Use existing bounded queues, SQL/argument bytes,
parameter count/value/total bytes, rows/elements/cells and encoded result budgets.
Count decoded binary vector bytes and encoded request overhead, not just argument
count. Never truncate an input vector or silently reduce K, probes or search
breadth. Report output truncation explicitly; a truncated ranking is not a full
top-K result. Distinguish approximate recall from output truncation.

Bound extension catalog objects/dependency edges, parser work and proof caches;
include their retained and transient allocations in the process memory forecast.
P1 must select tested numeric bounds alongside configuration validation. Bind
proof-cache keys to database/role/version/catalog revision, not SQL text alone.
Search tuning caps must account for backend memory and concurrency. No automatic
retry may restart a cancelled expensive search. Topology discovery retains the
existing Redis allowlist and attestation boundaries.

## Compatibility and migration

Existing SQL/Redis request shapes remain valid; new tuning is optional. Operators
install and populate indexes separately, then enable an exact reviewed profile.
Do not auto-upgrade extensions or generate a trusted profile from unreviewed live
metadata. Re-attest on version/definition changes; invalidate prepared/proof state.
Rollback disables the new profile and restores prior policy without data changes.
Version matrices must record server, extension/build, driver, dtype and index
combination; neither current upstream documentation nor skipped tests imply support.

## Observability

Expose implemented/qualified vector capability, profile identity and freshness.
Record content-free operation counts, proof failures, queue/pool wait, execution,
cancellation/recovery, response truncation and tuning-limit rejection. Do not log
vectors, query text, filters, parameters or returned payloads. Native score units
must be documented per engine/metric rather than normalized under a misleading
shared similarity field.

## Test and acceptance plan

1. Deterministic vector fixtures with independently calculated distances: exact
   search checks IDs, scores and ties; ANN checks declared recall criteria and
   valid scores. Query both through MCP and native baseline for comparison.
2. Redis: actual binary blobs containing zero/non-UTF-8 bytes, HASH/JSON, tested
   dtypes/metrics, FLAT/HNSW, KNN/range/filters, projections, native tuning, RESP2/3,
   profile expiry, alias/prefix drift and a qualified deployment topology.
3. PostgreSQL: approved extension startup, vector/halfvec parameters/results,
   exact/HNSW/IVFFlat applicable combinations, filtered iterative scans, joins,
   CTEs, aggregates, subqueries, expression indexes and reranking. Verify selected
   index use on representative data without forcing every valid query to use ANN.
4. Prove malicious overload/cast/type I/O/dependency rejection and native write
   denial. Test scope and RLS, profile/catalog drift, malformed dimensions and
   non-finite payloads, oversize requests/results and secret-free errors.
5. Verify 5/10/15-minute configuration bounds, successful long-running reads,
   deadline cancellation, whole-batch behavior, tuning reset, physical pool
   limits and recovery after timeout/network loss. Mix metadata/maintenance and
   interactive load; measure backend termination, RSS and connection counts.
6. Run repository race tests, vet/build, parser/binding differential tests and
   version-pinned real-server suites. Record unsupported combinations and skips
   separately; fixture-only passes cannot promote native vector capabilities.

## Rollout and implementation sequence

| Stage | Concrete work | Exit gate |
| --- | --- | --- |
| P0: Redis dense acceptance | Extend local fixtures with real vector indexes/blobs; add MCP round trip and profile provisioning examples; fix any observed transport/normalization issue | Exact/ANN/filter/range positives, native write denial and binary round trip on an exact pinned build |
| P1: PostgreSQL extension proof | Config/profile, catalog object closure, exact privilege exception, binding prototype and hostile corpus | Binding design proven against native resolution; untrusted overload/cast/library profile cannot pass |
| P2: PostgreSQL retrieval | Enable vector SQL/parameters/results and per-request local tuning across query/batch/validation/explain | Full planned dense positive corpus, native write denial, scope proof and zero settings leakage |
| P3: release qualification | Version/topology matrix, long queries/cancellation/load/memory, capability reporting and operator docs | Reproducible native evidence, resource measurements and rollout/rollback checklist |

P0 and P1 are technically independent. Prioritize P0 for the shortest verified
usable path, then P1/P2; do not expose PostgreSQL extension execution before P1.
Deploy first to disposable fixtures, then a reporting replica with capacity
headroom. Alert on stale proof, increased queue/pool waits, cancelled work still
running and backend memory growth; disable the profile if proof or recovery fails.

## Alternatives considered

- Installing plugins and declaring support: ignores startup checks and evidence.
- Adding `<->`/`FT.SEARCH` to a keyword whitelist: misses binding, casts and scope.
- Allowing all extension functions or trusting volatility: misses native effects.
- A fixed top-K tool replacing native SQL/Search: unnecessarily loses advanced reads.
- Arbitrary session SET: risks cross-call state leakage; typed transaction-local
  options cover retrieval tuning without that behavior.

## Completion criteria

All P0–P3 gates pass for each advertised profile. Examples work through the real
MCP server with a dedicated reader, advanced reads remain usable, persistent
writes fail at both layers, request resources recover, and the qualification
record lists actual evidence. Redis P0 now has local standalone dense evidence;
broader release qualification and pgvector support remain pending.

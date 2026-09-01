# RFC-0001: Performance and Resource Governance

- Status: Implemented; production MySQL load validation pending
- Authors: readonly-db-mcp maintainers
- Created: 2026-09-01
- Target release: staged
- Scope: MySQL targets and stdio transport

## Implementation status

Phases 1 through 6 are implemented. Phase 0 includes committed microbenchmarks
and an environment-driven disposable-MySQL integration suite. The integration,
mixed-load and soak portions still require deployment infrastructure and must be
run before production sizing is approved.

The exact response ledger uses conservative incremental row accounting followed
by exact final `encoding/json` verification. If adjustment is required, a binary
search selects the largest fitting row prefix. This preserves the RFC's exact
wire-size invariant while avoiding a full-response marshal after every row.

## Summary

This RFC introduces a resource-governance and performance architecture for all
database-backed MCP tools. It has six related changes:

1. one admission controller for query and metadata operations;
2. bounded, versioned metadata caches;
3. immutable target policy indexes built at startup;
4. separate workload classes with reserved capacity and fair scheduling;
5. phase-level metrics and structured audit coverage;
6. an exact response-budget ledger for single and batch queries.

An optional, explicitly enabled result cache is defined as a later phase. It is
not enabled by default because freshness and confidentiality are target-specific.

The design favors predictable tail latency and bounded resource use over maximum
throughput. No optimization may weaken SQL validation, grant attestation,
read-only transactions, target isolation, result limits, or error sanitization.

## Motivation

The current implementation has small and understandable execution paths, but
its controls do not cover every database operation uniformly:

- free-form queries use global and per-target semaphores, while schema metadata
  requests go directly to `database/sql`;
- metadata is fetched repeatedly even though it changes infrequently;
- allowed-schema and denied-table lookup maps are rebuilt on requests;
- the configured result-byte limit counts encoded rows but not column metadata,
  response envelopes, or encoding overhead;
- batch queries learn that the combined response is too large only after work
  has already been executed and materialized;
- the only duration currently exposed is end-to-end duration, so queueing,
  database work, scanning, and encoding cannot be distinguished;
- one expensive workload class can consume all shared capacity.

These properties are acceptable at small scale but make latency and memory less
predictable as targets, clients, schemas, and query concurrency grow.

## Goals

- Bound database concurrency across every public database-backed tool.
- Preserve useful metadata responsiveness while analytical queries are busy.
- Reduce repeated `information_schema` work by at least 80% on steady workloads.
- Make configured response limits conservative: actual serialized output must
  never exceed the configured budget.
- Keep process memory proportional to configured concurrency and byte limits.
- Expose enough measurements to explain p50, p95, and p99 latency regressions.
- Avoid serving cached data across targets, identities, or policy revisions.
- Make each phase independently deployable and reversible.

## Non-goals

- Replacing MySQL's optimizer or automatically rewriting user SQL.
- Adding a distributed cache or external queue.
- Increasing database privileges to obtain more metadata.
- Providing a freshness guarantee for replicas.
- Streaming partial MCP tool results in the first implementation. The current
  tool contract remains one bounded structured response.
- Persisting query results to disk.

## Performance objectives

The initial objectives are acceptance thresholds, not universal service-level
agreements. Maintainers should revise them using production baselines.

| Scenario | Objective |
| --- | --- |
| Policy validation, 32 KiB SQL | p95 below 5 ms on the reference CI runner |
| Warm metadata lookup | p95 below 2 ms, with no database round trip |
| Metadata cache steady state | at least 80% hit ratio after warm-up |
| Scheduler overhead without queueing | p95 below 250 microseconds |
| Query response encoding | at least 100 MiB/s for scalar tabular data |
| Budget enforcement | serialized response never exceeds configured limit |
| Overload behavior | bounded queue; fail before opening a DB transaction |
| Metadata under query saturation | p99 bounded by metadata queue timeout |
| Memory | no unbounded growth under a 30-minute mixed-load soak test |

End-to-end database latency depends on the target and is reported separately
from server overhead.

## Proposed architecture

```text
MCP handler
    |
    v
request validation + exact target lookup
    |
    v
workload classification
    |
    v
bounded fair admission controller
    |                    |
    | metadata           | query / explain / batch
    v                    v
metadata cache       SQL policy validation
    |                    |
    +------ cache miss   v
             |       read-only transaction
             v           |
          database <-----+
             |
             v
      response budget ledger
             |
             v
       structured result + metrics + audit
```

### 1. Unified admission controller

Every operation that can use a database connection must acquire an admission
permit before calling `database/sql`:

- `schema_list_tables`;
- `schema_describe_table`;
- `query_select`;
- `query_explain`;
- `query_batch`;
- startup and explicit health checks, using a separate maintenance class.

The controller owns both the global and per-target limits. Callers no longer
manipulate semaphore channels directly.

```go
type WorkClass string

const (
    WorkMetadata    WorkClass = "metadata"
    WorkInteractive WorkClass = "interactive"
    WorkBatch       WorkClass = "batch"
    WorkMaintenance WorkClass = "maintenance"
)

type Permit interface {
    Release()
}

type AdmissionController interface {
    Acquire(ctx context.Context, target string, class WorkClass) (Permit, error)
}
```

Requirements:

- acquisition is context-aware;
- permits are idempotently released;
- a bounded waiter count rejects overload instead of accumulating goroutines;
- queue timeout is independently configurable and cannot exceed request timeout;
- cancellation removes the waiter without leaking capacity;
- metrics identify class, target, admitted/rejected decision, and wait duration;
- target names may be included in logs but metrics must use an opt-in target label
  to avoid uncontrolled cardinality.

The first implementation should use a mutex-protected FIFO queue with class
reservations, not nested channel acquisition. This makes cancellation and
fairness explicit and avoids holding global capacity while waiting for target
capacity.

### 2. Workload classes and fairness

Metadata, interactive selects, and batches have different latency and resource
profiles. The scheduler therefore uses class reservations and weighted
round-robin among non-empty queues.

Default policy:

- reserve one global slot for metadata when global concurrency is at least 3;
- batch may use at most 25% of global capacity, rounded up;
- interactive work may borrow unused metadata capacity;
- metadata may borrow unused interactive capacity;
- maintenance work runs only when a slot is free and cannot consume the last
  interactive slot;
- reservations are work-conserving: idle capacity is not left unused;
- a continuously backlogged class must receive service within a bounded number
  of admissions.

Per-target capacity remains a hard ceiling. `connection.max_open` must be at
least the target admission ceiling; configuration validation rejects impossible
combinations. The default remains conservative and never raises existing pool
sizes automatically.

New configuration shape:

```yaml
limits:
  global_concurrency: 4
  per_target_concurrency: 2
  max_queued_requests: 32
  queue_timeout: 500ms
  workload_classes:
    metadata_reserved: 1
    batch_max_concurrency: 1
    maintenance_max_concurrency: 1
```

Existing configurations receive safe defaults. Unknown fields remain rejected.

### 3. Metadata cache

Each target owns a bounded in-memory metadata cache. Entries never cross target
boundaries.

Cache keys:

- table list: `(targetID, normalized LIKE pattern, policyRevision)`;
- table description: `(targetID, exact schema, exact table,
  policyRevision)`.

`targetID` is an internal immutable identifier, not only the public alias. The
policy revision is a hash of engine, database, allowed schemas, denied tables,
and metadata output version. This prevents reuse after a configuration or policy
change.

Default behavior:

- table-list TTL: 30 seconds;
- table-description TTL: 5 minutes;
- maximum entries per target: 256;
- maximum approximate bytes per target: 8 MiB;
- LRU eviction within a target;
- singleflight request coalescing for identical misses;
- successful, non-empty responses are cached;
- denied, timed-out, canceled, and database-error responses are not cached;
- a legitimate empty table list may be cached briefly, but a missing table
  description is negative-cached for no more than 5 seconds;
- returned values are cloned or immutable so callers cannot mutate cache state.

TTL expiry is lazy. No background refresh goroutine is required initially.
Serving expired entries is forbidden by default. A future stale-while-revalidate
mode would require a separate RFC because it changes freshness semantics.

Callers may set `fresh: true` on either metadata tool. This bypasses a cached
entry and atomically replaces it with a database read. Targets may disable this
capability, and a per-key cooldown plus miss coalescing prevents refresh storms.
Refresh failure is returned to the caller and never silently falls back to stale
metadata.

The cache itself does not consume an admission permit on a hit. A miss acquires
a metadata permit before accessing the database. Singleflight waiting obeys the
caller's context and does not create a second database request when one caller
cancels.

New configuration:

```yaml
targets:
  reporting-production:
    metadata_cache:
      enabled: true
      allow_fresh: true
      fresh_cooldown: 1s
      table_list_ttl: 20m
      table_description_ttl: 20m
      negative_ttl: 5s
      max_entries: 256
      max_bytes: 8388608
```

An internal invalidation method is provided for tests, future config reload, and
administrative health workflows. No public MCP cache-mutation tool is added.

### 4. Immutable target policy indexes

The target constructor builds immutable, normalized indexes once:

- `allowedSchemaSet`;
- unqualified and qualified denied-table sets;
- sorted allowed-schema parameter slice;
- prebuilt metadata query placeholders;
- policy revision hash;
- response/cache configuration validated against hard ceilings.

Request paths perform read-only lookups and allocate no normalization maps. This
also consolidates case-handling rules so SQL policy and metadata policy cannot
drift.

These structures are never mutated after target publication. If configuration
reload is later added, a new target snapshot is constructed and atomically
swapped rather than editing maps in place.

### 5. Exact response-budget ledger

The server introduces a budget object used during result construction:

```go
type ResponseBudget interface {
    ReserveEnvelope(meta ResponseMeta) error
    AddColumn(column core.Column) error
    AddRow(row map[string]any) error
    Remaining() int
    Used() int
}
```

The authoritative definition is the byte length of the final JSON-compatible
structured result encoded by the MCP SDK. To avoid encoding the entire response
after every row, the implementation will use a counting JSON writer with the
same escaping behavior as `encoding/json` and reserve fixed envelope punctuation
up front. Tests compare the incremental count to an actual final marshal for
every result shape.

Rules:

- column metadata and the response envelope consume budget;
- rows are admitted only when their complete encoded representation fits;
- cell truncation occurs before row accounting;
- when the next row cannot fit, the result is returned with `truncated=true`;
- a budget too small for the envelope and columns returns a bounded error;
- maximum column count is introduced to bound scan allocations;
- a maximum in-memory result budget may be lower than the wire limit to cover Go
  object overhead; initially use a conservative configurable multiplier;
- audit records include budget used and truncation reason, never values.

For a batch, one parent ledger owns the entire response. It reserves the batch
envelope and per-result metadata before each query. `collectRows` receives a
child view of the remaining parent budget. Later queries therefore cannot each
consume the full single-query limit.

Before executing each batch item:

1. verify enough budget remains for a minimal result envelope;
2. otherwise stop without executing that item and return a successful truncated
   batch with an explicit `completed_queries` count;
3. execute and collect against the remaining budget;
4. return unused reservation to the parent.

This is a response-schema change. To avoid ambiguous partial results, the batch
result gains:

```json
{
  "truncated": true,
  "truncation_reason": "result_byte_limit",
  "completed_queries": 3
}
```

The change must be documented in the tool schema and release notes. Existing
fields retain their meaning.

### 6. Phase-level observability

Instrumentation records the following durations separately:

- request/schema validation;
- SQL parse and policy walk;
- admission queue wait;
- connection/transaction begin;
- database query until first row;
- row scan and normalization;
- response budget accounting and encoding;
- rollback/cleanup;
- total request.

Required counters and gauges:

- requests by operation, class, and outcome;
- active permits and queued requests by class;
- admission rejection and queue timeout counts;
- metadata cache hits, misses, coalesced waits, evictions, entries, and bytes;
- database errors by sanitized class;
- rows returned, truncated rows/cells, and response bytes;
- pool statistics sampled from `sql.DB.Stats()`;
- grant/health verification status and age.

The stdio server must not write metrics to stdout. The initial exporter is a
periodic structured summary on stderr, disabled by default. The internal metrics
interface must allow a future OpenTelemetry or Prometheus adapter without adding
those dependencies to core execution code.

```go
type Recorder interface {
    ObserveDuration(name string, duration time.Duration, attrs Attributes)
    AddCounter(name string, delta int64, attrs Attributes)
    SetGauge(name string, value int64, attrs Attributes)
}
```

Attributes use controlled enums. Raw SQL, parameters, result values, credentials,
hosts, usernames, schema names, table names, query IDs, and fingerprints are
forbidden as metric labels. Fingerprints and audited table names remain confined
to structured audit events.

Audit coverage is expanded to include rejected input, admission rejection,
queue timeout, transaction-begin failure, collection failure, budget truncation,
and cancellation. Audit recording stays synchronous only while the current
`slog` implementation is non-blocking enough for deployment. If an asynchronous
sink is added, it must be bounded and report dropped-event counts.

### 7. Optional query-result cache

Result caching is a separate, final implementation phase and defaults to off.
It is allowed only when a target explicitly declares acceptable staleness.

Eligibility:

- target configuration enables it and defines a positive TTL;
- consistency is `eventual`, unless the operator explicitly overrides it;
- operation is `query_select`, never batch or explain initially;
- SQL policy validation succeeds;
- the query contains no volatile expression such as `RAND`, current time/date,
  UUID generation, connection/session identity, or other engine-classified
  nondeterminism;
- result was successful and was not truncated;
- encoded result is below the per-entry ceiling.

Key material:

- internal target ID and policy revision;
- canonical query fingerprint;
- typed, length-delimited parameter encoding;
- max rows and response-shaping options;
- result schema version.

Keys use SHA-256; raw SQL and parameters are not retained in cache metadata.
Values remain in process memory only. Entries are bounded by TTL, count, total
bytes, and per-entry bytes. The cache is target-local and is completely flushed
when a target closes or its policy revision changes.

```yaml
targets:
  reporting-production:
    result_cache:
      enabled: false
      ttl: 10s
      max_entries: 128
      max_bytes: 16777216
      max_entry_bytes: 262144
      allow_current_consistency: false
```

Responses expose `cache_status` (`hit`, `miss`, `bypass`) and `cache_age_ms` on a
hit so callers are not misled about freshness. Audit events record cache status
but never cache key material.

## Configuration validation and hard ceilings

New settings remain subject to compiled hard ceilings:

- queued requests: 1 to 1024;
- queue timeout: 1 ms to 30 seconds and no more than maximum query timeout;
- metadata cache: at most 10,000 entries and 256 MiB per target;
- result cache: at most 10,000 entries and 512 MiB per target;
- aggregate configured cache budget: at most a process-level ceiling;
- response size and column count retain conservative hard limits.

The process computes a startup resource forecast:

```text
connection ceiling + admission ceiling + queue ceiling
+ metadata cache bytes + result cache bytes
+ concurrent response memory allowance
```

Configuration is rejected when internally inconsistent. The forecast is logged
without secret connection details.

## Failure behavior

- Admission overload returns a sanitized `server busy` error with a stable error
  class; it does not open a transaction.
- Cache failure is fail-open to the underlying database only when admission and
  deadline permit it. Cache accounting corruption disables that target's cache
  and emits an error.
- Metric/export failure never fails a query.
- Audit sink behavior remains explicit; a future compliance mode may choose
  fail-closed audit semantics, but that is outside this RFC.
- Deadline and cancellation always take precedence over cache fill or metrics.
- A panic in cache/scheduler code is not recovered in the request path merely to
  continue with potentially corrupted accounting; normal process supervision
  should restart the server.

## Security and privacy considerations

- Cached database values increase their lifetime in process memory. Metadata
  caching is enabled by default; result caching is opt-in and memory-only.
- Cache keys must not permit cross-target or cross-policy reuse.
- Metrics have a strict low-cardinality label policy and contain no database
  content.
- Cached results are cloned/immutable to avoid accidental mutation and leakage.
- Optimizations do not bypass SQL validation or grant verification. A result
  cache lookup occurs only after exact target lookup and request validation; its
  eligibility decision uses the validated AST.
- The admission controller covers metadata to close the current resource-control
  bypass.

## Compatibility

- Existing configuration files remain valid through defaults.
- Tool names and existing request fields do not change.
- Metadata content is unchanged; only freshness becomes bounded by documented
  TTLs. Operators can disable metadata caching per target.
- Single-query response fields remain compatible, with optional cache metadata
  added only when result caching is implemented.
- Batch adds explicit partial-completion fields. This is additive at JSON level,
  but clients assuming every requested item is always present must be updated.

## Implementation plan

### Phase 0: Baseline and test harness

- Add deterministic benchmarks for policy validation, row normalization,
  response encoding, and metadata-policy lookups.
- Add MySQL 8 integration tests for cancellation, connection reuse, read-only
  transactions, metadata load, and batch limits.
- Add a mixed-workload load generator and a 30-minute soak profile.
- Capture CPU, allocations, heap high-water mark, goroutines, pool waits, and
  latency percentiles before code changes.

Exit criteria: reproducible baseline report committed under `docs/benchmarks/`.

### Phase 1: Immutable policy indexes and instrumentation skeleton

- Precompute target lookup sets and metadata query fragments.
- Introduce no-op metrics interfaces and phase timers.
- Preserve behavior exactly.

Exit criteria: no output changes; allocation reduction demonstrated in
microbenchmarks; unit and integration suites pass.

### Phase 2: Unified admission and workload isolation

- Replace semaphore channels with the admission controller.
- Route metadata, query, explain, batch, and maintenance operations through it.
- Add bounded queues, cancellation tests, fairness tests, and overload tests.

Exit criteria: no connection access outside admitted/startup paths; race tests
pass; metadata remains responsive under batch saturation.

### Phase 3: Metadata cache

- Add target-local bounded LRU, byte accounting, TTLs, and miss coalescing.
- Instrument hit ratio and evictions.
- Add concurrency, cancellation, mutation-safety, and policy-revision tests.

Exit criteria: at least 80% DB-call reduction in the reference warm workload;
bounded memory in soak tests.

### Phase 4: Exact response budgets and batch early stop

- Implement the counting encoder and parent/child ledgers.
- Include columns and envelopes in accounting.
- Add batch partial-completion fields and documentation.
- Add property/fuzz tests comparing incremental and final encoded byte counts.

Exit criteria: no generated response exceeds its configured limit across fuzz
corpora; batch does not execute items that cannot fit a minimal response.

### Phase 5: Operational metrics and health reporting

- Add periodic stderr summaries and `sql.DB.Stats()` sampling.
- Complete failure-path audit events.
- Document recommended dashboards and alerts.

Exit criteria: every latency phase and overload path is distinguishable without
logging database content.

### Phase 6: Optional result cache

- Implement AST volatility classification.
- Add bounded target-local result caches with explicit freshness metadata.
- Run correctness tests against changing source data and policy revisions.

Exit criteria: disabled by default; zero cross-target/policy hits; demonstrated
latency/DB-load benefit on an explicitly eventual-consistency workload.

## Test strategy

Unit tests:

- scheduler capacity, FIFO behavior, weighted fairness, borrowing, cancellation,
  overload, and idempotent release;
- cache TTL, LRU and byte eviction, singleflight, negative cache, cloning, and
  policy revision isolation;
- exact budget accounting for escaping, Unicode, binary/base64 values, duplicate
  columns, large integers, nulls, and empty results;
- deterministic/volatile AST classification;
- configuration defaults, validation, and hard ceilings.

Property and fuzz tests:

- incremental byte accounting always equals final encoding;
- arbitrary cancellation schedules never leak permits;
- cache operations never exceed entry or byte ceilings;
- arbitrary queries cannot make volatile results cache-eligible.

Integration tests with disposable MySQL 8:

- actual simultaneous connection count never exceeds admission/pool ceilings;
- metadata calls cannot bypass saturation controls;
- canceled queries free or discard connections promptly;
- batch snapshot semantics remain unchanged;
- grants and read-only transaction guarantees remain intact;
- cache invalidation and TTL behavior reflect schema/data changes as documented.

Load tests:

- 70% interactive query, 20% metadata, 10% batch;
- metadata storm;
- slow-query saturation;
- many targets with low per-target concurrency;
- maximum-size cells and responses;
- cache stampede after synchronized expiry;
- 30-minute steady load followed by idle recovery.

All concurrency changes must pass `go test -race ./...` in a CI environment with
a working race runtime. The current development toolchain should not be treated
as sufficient until that command succeeds.

## Benchmark reporting

Each phase records:

- commit and Go/MySQL versions;
- CPU and memory limits;
- configuration and dataset scale;
- operations per second;
- p50/p95/p99 total and phase latency;
- allocations and bytes per operation;
- peak heap, goroutines, open/in-use connections, and queue depth;
- cache hit/miss/eviction statistics;
- error, timeout, rejection, and truncation rates.

Results are compared to Phase 0. A phase must not merge if it introduces more
than a 5% statistically repeatable regression in an unaffected critical path
without an explicit accepted tradeoff.

## Rollout and rollback

1. Ship metrics and precomputed indexes first.
2. Enable unified admission with defaults equivalent to current query limits.
3. Enable metadata caching on test targets, then production replicas.
4. Enable exact response budgets and announce batch partial-completion fields.
5. Keep result caching disabled until target owners explicitly opt in.

Feature switches are per capability, not one global "performance mode". Rolling
back a cache clears its in-memory state. Rolling back admission restores the
previous controller only during the staged release window; after metadata is
covered by the new controller, bypass behavior must not become a supported mode.

## Operational alerts

Recommended alerts after baselining:

- admission rejection rate above 1% for 5 minutes;
- p95 queue wait above 50% of queue timeout;
- metadata hit ratio below 60% after warm-up;
- cache evictions continuously above fill rate expectations;
- DB pool wait count or duration increasing while admission has free capacity;
- response truncation above the workload's expected rate;
- health/grant verification age beyond the configured interval;
- heap remaining above twice the configured cache plus response forecast after
  an idle recovery window.

## Alternatives considered

### Only increase connection pool sizes

Rejected. It can raise throughput temporarily but increases database load and
does not bound queues, memory, metadata storms, or batch monopolization.

### One semaphore for every operation

Rejected as the final design. It closes the metadata bypass but permits batches
to starve latency-sensitive metadata and interactive queries.

### External Redis cache

Rejected for this stage. It adds network, authentication, invalidation, and data
retention risks to a local-first server. The expected working set fits bounded
process memory.

### Cache all successful SELECTs

Rejected. SQL volatility, replica freshness, policy changes, and sensitive data
retention require explicit eligibility and operator consent.

### Estimate memory using row JSON only

Rejected. It is the current approximation and does not conservatively bound the
serialized response or Go heap usage.

## Open decisions for implementation review

The RFC recommends defaults, but the implementation PR must confirm these with
baseline evidence:

1. whether weighted round-robin or deficit round-robin produces better fairness
   with batch weights;
2. the conservative heap-to-wire multiplier for response memory forecasting;
3. whether the MCP SDK exposes a counting path identical enough to avoid a final
   validation marshal;
4. default metadata TTLs for production schemas with frequent migrations;
5. whether periodic grant re-attestation belongs in the maintenance class in the
   same release or a follow-up security RFC.

## Acceptance criteria

This RFC is complete when Phases 0 through 5 are implemented and:

- every runtime database call is governed by admission control;
- overload creates neither unbounded goroutines nor unbounded memory;
- warm metadata avoids database access and honors configured freshness;
- target policy lookup performs no per-request map construction;
- final structured responses never exceed the configured byte ceiling;
- batches stop before executing work that cannot be represented in the budget;
- phase latency, queues, pools, cache, budget, and failures are observable;
- unit, fuzz, integration, race, load, and soak tests pass;
- security controls and existing read-only behavior remain unchanged.

Phase 6 is optional and accepted separately because it changes freshness and
data-retention characteristics.

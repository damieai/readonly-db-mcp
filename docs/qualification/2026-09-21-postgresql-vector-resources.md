# PostgreSQL dense retrieval — P3 local recall and recovery evidence

This adds reproducible local acceptance to the [P2 functional matrix](2026-09-21-postgresql-vector-retrieval.md).
It does not complete production release qualification. The fixture uses the same
PostgreSQL 16.2 / pgvector 0.8.2 artifacts and native proof helper recorded in
[P1](2026-09-21-postgresql-vector-proof.md), on Linux x86-64 with a private Unix
socket. These historical fixture builds are not a production version recommendation.
The Go driver is pgx v5.10.0 and the MCP SDK is v1.6.1.

## Recall and native SQL compositions

`TestLocalPostgreSQLVectorRecall` provisions 16,384 16-dimensional vectors from a
fixed seed. Components are multiples of 1/1024 in [-1,1], exactly representable
in both `vector` and `halfvec`. A separate Go Euclidean-distance calculation
supplies the ranking oracle; a native exact SQL scan also checks its IDs and
scores. Exact baseline SQL sorts by distance plus zero to avoid an ANN scan.
Equal distances at the top-K boundary are accepted as ties.
The shared exactly representable data tests reranking composition, not a recall
gain from correcting half-precision quantization.

The 36 retrieval cases comprise two dense types, exact/HNSW/IVFFlat, and six
scenarios: four independent query vectors, a filter selecting one eighth of the
corpus, and a materialized 100-candidate CTE followed by exact reranking. Every
case checks ten complete, unique results, valid filter membership, sorted scores
and absolute score error within 1e-5. Exact recall@10 must be 1.0; each ANN case
must reach at least 0.9, rather than hiding a weak query in a mean.

HNSW uses `ef_search=100` and strict iterative scanning, with relaxed scanning
for the reranking case. IVFFlat has 32 lists, 24 probes, a 32-probe iterative
ceiling and relaxed iterative scanning. These are declared test inputs, not new
runtime defaults or imposed limits on user tuning. PostgreSQL's ordinary
sequential-scan preference is restored; MCP EXPLAIN must show the actual ANN
index for every approximate scenario. JIT remains disabled in the fixture.
Broad probing (24 of 32 lists) and high-recall settings on this synthetic,
low-dimensional corpus do not establish production recall or latency.

## Native interruption, cleanup and connection reuse

`TestLocalPostgreSQLVectorRecovery` submits an expensive vector-distance
aggregation through MCP and observes its active reader backend through
`pg_stat_activity` for at least 100 ms, excluding lock waits, before testing:

- Client cancellation through the MCP SDK, with a queued 350 ms request expiring
  while the only admission permit is occupied.
- A 5-second tool deadline during native execution.
- Administrative termination of the observed disposable backend, simulating a
  lost backend connection; the failed query is not transparently retried.
- A 5-second whole-batch deadline. An administrator-held table lock consumes
  two seconds during the first query's admission/binding/execution path; the
  second query must actually start and the entire batch must still terminate
  within 6.5 seconds. The deadline is not renewed for the second item.

Each case waits up to five seconds for all MCP reader sessions to become idle
without an open transaction, or disappear. A subsequent real vector expression
must succeed with default HNSW/IVFFlat settings restored. The expression also
loads pgvector on a replacement backend before reading its native settings.
Reading extension GUCs alone on a fresh backend would not test vector recovery.

Eight concurrent follow-up MCP calls must all complete. During this phase a
10 ms observer samples the physical reader backend count and rejects a count
above the configured single connection. The separate native reader returned by
the fixture is never connected in this test, so observed reader sessions belong
to MCP, including catalog/helper work. This is a sampled local ceiling check,
not a claim about unobservably short connection overlaps or multi-target fairness.

The test logs Linux backend RSS when native work is observed. These are point
samples, not peak measurements, memory bounds, or MCP process memory accounting.
The expensive aggregate exercises the vector execution path and cancellation;
it does not qualify cancellation inside every ANN access-method phase.

## Reproduction

Install the pinned pgvector build and build the helper as described in the
[operator guide](../POSTGRESQL-VECTOR-RETRIEVAL.md). With absolute artifact paths:

```sh
export READONLY_DB_MCP_PG_BIN=/absolute/path/to/postgresql16/bin
export READONLY_DB_MCP_PGVECTOR_PROOF="$PWD/bin/pgvector-proof.so"
make test-postgresql-vector-qualification
# Also run the original 22-combination functional and rejection matrix:
make test-postgresql-vectors
```

Both targets require the fixture variables and run race-enabled tests. No
existing database is modified and no external service is contacted. Direct Go
invocations skip these native suites when the artifact variables are absent.

## Observed validation

On 2026-09-21, the standalone recall race suite passed in 79.795 seconds.
The subsequent `go test -race ./... -count=1 -v` passed with the native fixture
variables set (MCP package: 176.860 seconds; binding/proof package: 16.312 seconds).
All 36 recall cases measured recall@10 of 1.0 in both runs, including the 24
ANN cases with verified index plans. This result applies to the declared corpus
and settings; approximate indexes remain approximate.

The full run also passed the original 22-combination retrieval/rejection matrix
and all four interruption/recovery cases. The concurrent recovery phase completed
all eight requests with a sampled peak of one reader backend. `go vet ./...`,
`make build`, document-link and whitespace checks passed. Other engines' native
environment-gated fixtures were not enabled; their skips are not qualification.

After changing the cancellation workload to join two small series (so it enters
repeated vector-distance evaluation promptly), the final recovery race suite
passed again in 27.433 seconds:

| Case | MCP request elapsed | Cleanup observed | Backend RSS point sample |
| --- | --- | --- | --- |
| Client cancellation | 1.708 s | 12.4 ms after cancel | 20,368 KiB |
| Tool deadline | 4.753 s | 1.2 ms after MCP error | 21,132 KiB |
| Backend termination | 1.266 s | 1.5 ms after termination trigger | 21,708 KiB |
| Whole-batch deadline | 4.753 s | 0.4 ms after MCP error | 21,184 KiB |

The final run again completed eight concurrent recovery requests with a sampled
peak of one backend. Cleanup timing includes observation overhead and uses the
different starting events stated above; these are measurements on this host,
not production latency or memory guarantees.

## Remaining P3 gates

Current production builds, TCP/TLS and deployment topology; representative
high-dimensional datasets and additional metric recall; successful multi-minute
reads; sustained mixed metadata/maintenance/interactive fairness; larger pool
ceilings; peak backend/MCP RSS and temporary-file budgets; ANN-specific
cancellation phases; network blackholes, TLS reconnect and failover remain
unqualified. Administrative backend termination is not a network-fault test.
The operator-asserted binary identity and trusted-administrator boundaries from
P1/P2 remain unchanged.

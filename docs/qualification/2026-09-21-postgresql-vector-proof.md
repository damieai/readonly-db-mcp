# PostgreSQL vector proof foundation — RFC-0007 P1

This is native evidence for an internal catalog/binding prototype. P1 is still
in progress; PostgreSQL vector retrieval through MCP is not enabled or qualified.

## Fixture identity

- PostgreSQL 16.2, Linux x86-64, from a locally cached `pgserver` installation
  copied to a disposable prefix. This historical test binary is not a production
  version recommendation or evidence for every PostgreSQL 16 patch release.
- pgvector 0.8.2, built from source commit
  `cab9da72c04353f143bb06b42ab70a403daac64a` with `OPTFLAGS=`.
- Native helper compiled against those server headers with warnings as errors.
- Every native run initializes a fresh cluster, private Unix socket, fixture
  owner and reader. Persistent data is confined to temporary test directories.

| Artifact | SHA-256 |
| --- | --- |
| pgvector `sql/vector.sql`, LF-normalized | `7fb5bb279ef83bf9204bfac7405bb5c9a05e49f5ac7d64d1eb4464d103b80f32` |
| Locally built `vector.so` | `a2f7a09c3773cdf0d8116365b1269b3087abdd2dd2c7ec23cbcbbab6982791a2` |
| Locally built `pgvector-proof.so` | `3a2817236bfbd1cd3ab0adf3474078edfe18bad2dd931ada68407884eba666bd` |

The source-derived profile has 327 descriptors: 118 procedures, 54 operator
family support entries, 40 operators, 36 operator family strategy entries,
24 operator classes, 24 operator families, 23 casts, six types and two access
methods. Actual OIDs are resolved from each fresh database. A changed installation
namespace matches the same profile. These descriptors do not authenticate a
remotely loaded native library.

## Assertions covered

`TestLocalPGVectorCatalogAndBinding` checks:

- Exact catalog comparison and effective reader EXECUTE privileges, including
  PUBLIC grants. An unreviewed executable routine is rejected even if it is in
  the approved extension's schema.
- Ten native SQL binding cases: KNN operators, explicit operator/function
  qualification, CTE joins and windows, vector aggregates, LATERAL, halfvec and
  array casts, recursive CTE/JSON expressions, filtered/ordered-set aggregates.
  Each operator's resolved procedure is checked against PostgreSQL's catalog.
- SQL, database and role identity mismatches. A view exposing a disallowed base
  schema fails relation scope. Under the dedicated reader, native rewriting
  includes the RLS function; replacing it with an unreviewed routine is rejected.
- Eight catalog drifts: function/aggregate security-definer, volatility, removed/added extension
  membership, added HNSW family support, changed type input and removed cast
  membership. These are catalog-rejection tests, not execution of hostile code.
- Unreviewed overload, operator and cast resolution, as well as effective grant
  rejection. The adversarial routines are harmless owned fixture functions.
- Multiple statements, DML, modifying CTE, row locking, SELECT INTO and excessive
  parameter numbers are rejected by the native helper.
- Profile relocation, strict binding envelopes, class-qualified OID collisions,
  snapshot-copy isolation and removal of verified state after failed comparison.

The positive corpus proves native analysis and expression inspection. It does
not prove retrieval distances, ANN recall, result codecs or a complete execution
dependency closure. Some analysis cases use the fixture owner; RLS and effective
privilege cases explicitly use the reader.

## Reproduction and checks

Follow the [helper build instructions](../../tools/pgvector-proof/README.md),
then run `make test-pgvector-proof`. The native test requires both absolute path
environment variables. Run `go test -race ./...`, `go vet ./...`, `make build`
and `git diff --check` for repository validation. Other engine suites that lack
their fixture artifacts may skip and are not promoted by this record.

Observed 2026-09-21: the final repository race run passed with both PostgreSQL
fixture variables configured; the vectorproof package completed in 2.732 seconds.
The native suite exercised all eight catalog drifts and the advanced binding
corpus. Vet, binary build and whitespace checks passed. Redis, SQL Server and
other environment-gated native suites were not enabled in this run.

## Remaining gates

Before any runtime exception permits pgvector functions, finish analysis-time
type/typmod callback admission, full relation/index/domain/collation dependency
closure, helper and deployment identity, role/scope checks and freshness on the
execution lease. Then integrate the opt-in profile, native vector parameters and
results, request-local tuning and common query/batch/explain behavior. Full
exact/HNSW/IVFFlat retrieval, persistent-write denial, cancellation, pool reset,
load and memory qualification remain in RFC-0007 P2/P3.

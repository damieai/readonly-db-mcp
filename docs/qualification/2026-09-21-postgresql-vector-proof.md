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
| Locally built `pgvector-proof.so` | `1126c701edfb2d53920a487eeb82bda4c7f4055265cf58bd2220b20b8e3b973d` |
| Test-only `pgvector-test-fixture.so` | `03059784bd1088a1e5e57156b52dc4961f8886c98c5d3459831a41c062f274a7` |

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

`TestLocalPGVectorPreAnalysisDependencies` additionally exercises the combined
preflight/binding path under the dedicated reader:

- Eleven positive shapes cover dense KNN, CTE/window/join, aggregation, LATERAL,
  halfvec, domain/enum/array/composite/range types, collation/row comparison,
  partitioned tables, TABLESAMPLE, materialized views and window RANGE frames
  (including their implicit support-function identity). The fixture includes
  HNSW, expression and partial indexes, a view, and RLS.
- Eleven callback cases cover type input, revoked type USAGE, typmod, domain
  checks, expression/partial indexes, RLS, range canonicalization, TABLESAMPLE,
  subscripting and an inaccessible argument type reached via a visible routine.
  Each preflight failure leaves a native callback counter unchanged. A separate
  unsafe baseline reaches that callback and increments the same counter before
  throwing an error. The counter survives savepoint rollback, so an attempted
  callback cannot hide behind rollback or a native read-only error.
  Type input, revoked type USAGE, typmod and hidden-argument baselines run as the
  reader too; other baseline calls use the fixture owner to reach the callback
  independently of their revoked direct EXECUTE grants.
- A policy containing the pinned builtin `set_config` is rejected by inspecting
  its stored tree, despite pinned references being omitted from `pg_depend`.
- Changing an extension routine after capture invalidates the earlier comparison
  before analysis. Successful analysis restores the catalog-only search path.
- Dependency cycles deduplicate class-qualified objects, while repeated edges
  still consume the bounded traversal budget.

The graph follows type callbacks and composition, domain checks, range support,
view/SELECT-policy expressions, table constraints, partition keys/children,
index expressions/predicates, access methods, operator classes/families and
collations. It inventories the role's accessible analysis surface and readable
target relations, which may reject an unreviewed dependency even when one query
does not name it. Scope provisioning must account for this target-wide check.
It does not follow a materialized view's defining query as an execution path.
Other planner/executor mechanisms still need runtime admission and qualification;
these tests do not certify a complete production execution boundary.

## Reproduction and checks

Follow the [helper build instructions](../../tools/pgvector-proof/README.md),
then run `make test-pgvector-proof`. The native test requires both absolute path
environment variables. Run `go test -race ./...`, `go vet ./...`, `make build`
and `git diff --check` for repository validation. Other engine suites that lack
their fixture artifacts may skip and are not promoted by this record.

Observed 2026-09-21: the repository race run passed with both PostgreSQL fixture
variables configured (vectorproof: 15.445 seconds). After adding explicit window
RANGE support references, the complete affected package race suite passed again
in 15.448 seconds; the four reader-role coercion baselines also passed after
tightening their execution identity. Vet, binary build and whitespace checks
passed. Redis, SQL Server and other environment-gated native suites were not
enabled in this run.

## Remaining gates

Before any runtime exception permits pgvector functions, integrate helper and
deployment identity, effective role/scope checks, and freshness through planning
and execution on the same lease. The new preflight is internal; public query
admission still has no pgvector exception. Then integrate the opt-in profile, native vector parameters and
results, request-local tuning and common query/batch/explain behavior. Full
exact/HNSW/IVFFlat retrieval, persistent-write denial, cancellation, pool reset,
load and memory qualification remain in RFC-0007 P2/P3.

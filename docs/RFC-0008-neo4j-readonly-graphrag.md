# RFC-0008: Neo4j read-only GraphRAG connector

- Status: Disposable Enterprise 2026.09.0 gate passed; production TLS and sustained-read checks pending
- Created: 2026-10-04
- Depends on: RFC-0001 and the agent application data roadmap
- Workload: vector or full-text retrieval followed by bounded graph traversal and source citation

## Summary

Add one explicitly configured Neo4j Enterprise graph target and a native
`neo4j_read` MCP tool. A caller supplies Cypher and parameters, not a server,
database, credentials, or index creation request. Preserve Cypher's read
surface: pattern matching, variable-length traversal, subqueries, aggregation,
full-text reads, and native vector `SEARCH` with filters. This is a graph
connector, not a SQL dialect. Native Enterprise privileges are the persistent
write and graph-scope boundary; driver read routing is only a performance hint.

## Context and problem

The repository covers relational, key-value, document, vector and search data,
but has no graph traversal target. A GraphRAG agent needs to retrieve candidate
chunks or entities and then follow their relationships to cited source nodes
within one authorized graph. Neo4j's current Cypher supports vector `SEARCH`
without a legacy procedure and can combine it with graph traversal. The
2026.01 release introduced `SEARCH`; the older vector procedures were
deprecated in 2026.04 ([vector index manual](https://neo4j.com/docs/cypher-manual/current/indexes/semantic-indexes/vector-indexes/)).

Community Edition cannot satisfy this project's native least-privilege gate:
its users have implied administrator privileges
([user management manual](https://neo4j.com/docs/operations-manual/current/authentication-authorization/manage-users/)).
The Go driver's read access mode is for routing and explicitly is not an
authorization guarantee
([transactions manual](https://neo4j.com/docs/go-manual/current/transactions/)).

## Goals

- Return bounded, typed graph results with source IDs for a named database.
- Preserve read-only Cypher including Cypher 25 `SEARCH` and graph composition.
- Prove the current identity has no write, `LOAD`, boosted execution, admin,
  wildcard graph, or out-of-scope database grants before serving requests and
  during periodic rechecks.
- Show native denial of writes, cross-graph reads and outbound loads on a
  disposable Enterprise build, with cancellation and connection recovery.

## Non-goals

Graph writes, index creation, embedding generation, Neo4j Community Edition,
impersonation, arbitrary server-side plugins and multi-database federation are
outside this profile. Excluding those effects does not imply restricting
variable-length paths, `WITH`, `CALL {}` subqueries, union, aggregation, vector
`SEARCH`, or filters within the authorized graph.

## Proposed design

Use the official Go driver v6, pinned to one reviewed release, with TLS
verification and one operator-configured Bolt origin. Pin an exact Enterprise
server patch in configuration. No URI or database
switching comes from tool arguments. One session and read transaction serve a
request; the request context covers admission, privilege recheck, execution,
streaming and cleanup. Parameter values are passed through the driver without
Cypher interpolation. The server returns graph values in a bounded JSON shape:
nodes and relationships carry element IDs, labels/types and selected
properties; paths preserve ordered elements. Large integers retain decimal
precision in JSON. The connector must not silently stringify every value.

The target reports labels, relationship types and index/constraint
metadata through fixed native read commands. Vector index name, labels,
properties, dimension, similarity, state and provider are surfaced so clients
can form a valid native `SEARCH` query. Metadata reads use the same database
and admission limits. One MCP tool accepts one Cypher statement plus typed
parameters and optional row/timeout ceilings; a separate batch tool may be
added after snapshot and byte accounting are proved.

## Security and trust boundaries

Only Neo4j Enterprise 2026.03+ with a dedicated native-auth query user is admitted.
The user is assigned a custom graph-read role scoped to one exact database.
A separate attestor identity, limited to fixed system-directory reads, collects
`SHOW USER`, `SHOW USER PRIVILEGES`, and `SHOW AUTH RULES` at startup and on
expiry. The query identity separately runs `SHOW CURRENT USER`; its reported
user and roles must match the attestor's view. Refuse missing roles (Community), unknown fields,
unassigned-role rows, broad graph/database grants, any granted write/admin/
`LOAD`/unreviewed procedure or boosted-execution action, and future unreviewed actions.
`DENIED` rows may narrow authority, but do not excuse an unsafe `GRANTED` row
under this initial profile. The first policy implementation is
[`authority.Validate`](../internal/dialects/neo4j/authority/proof.go); the live
connector binds it to server-reported records; versioned live output tests remain pending.
Neo4j documents `SHOW USER PRIVILEGES` and its required inspection privileges
in the [RBAC manual](https://neo4j.com/docs/operations-manual/current/authentication-authorization/manage-privileges/).

Administrators must remove any broad `PUBLIC` grants that make the query
identity fail the proof. The connector does not silently ignore `PUBLIC`,
inherited roles, an auth-rule grant, or a grant on another graph. Enabled auth
rules with assigned roles are rejected because they can change effective roles
without changing the native user's assigned roles. Proof expires
on a bounded timer; failed recheck makes the target unhealthy and blocks new
queries. The live gate must demonstrate that revoking the read role and adding
a write grant both stop new calls. The query account itself remains unable to
persist writes even when a caller submits write Cypher. The driver accepts one
statement per call; administrative statements are denied by native grants.
No keyword allowlist is the security boundary.

`LOAD CSV` needs native load privileges, so none may be granted. Current
vector and full-text retrieval use native Cypher `SEARCH`, so neither needs a
procedure grant. Legacy `db.index.fulltext.queryNodes` and
`queryRelationships` procedures remain outside this profile until their exact
installed READ mode and absence of boosted execution are proved; a wildcard
execute grant remains invalid. Other procedures and user-defined functions
require a separate versioned profile and installed-code review. Ordinary `CALL {}` read
subqueries remain available because they are not procedure grants. See Neo4j's
[procedure privileges](https://neo4j.com/docs/operations-manual/current/authentication-authorization/manage-execute-permissions/)
and [full-text syntax](https://neo4j.com/docs/cypher-manual/current/indexes/syntax/).

## Latency, admission, and pool budget

Inherit the [common agent workload contract](ARCHITECTURE.md#agent-workload-latency-budget):
5-minute default query deadline, 10-minute configurable maximum, 15-minute
compiled ceiling, 1-minute default queue wait, 16/8 global/per-target
concurrency and 8/4 open/idle pool. A driver maximum connection pool size must
be set to the configured hard cap; connection acquisition and retries cannot
outlive the remaining request deadline. The query deadline includes proof
checks, result streaming and cleanup. Driver fetch-size is bounded. Agent
reasoning between calls holds no session, permit or transaction. Cancellation
must stop server work and release a permit; a subsequent query must succeed.
The live fixture must measure an ordinary and a multi-minute graph traversal
before any tighter default is proposed.

## Resource and response budget

Use the existing request SQL-text-equivalent, parameter, row, cell and final
response byte ceilings. Preflight Cypher text has a byte bound. Each streamed
record is normalized and measured before appending; one extra record marks
truncation. Bound nested path depth and property/list sizes without dropping
supported Cypher read constructs. Metadata and proof snapshots have separate
row/byte caps; unknown or partial proof is a startup/recheck failure. Audit
records include a query fingerprint and counts, never Cypher text, parameters,
property values or credentials.

## Compatibility and migration

The connector adds a new engine and tools, so existing targets retain their
behavior. The first target profile will require a pinned Enterprise patch,
verified TLS and a provisioned custom role. Upgrading Neo4j or the Go driver
requires a new privilege-output and hostile-query qualification run. A target
that fails proof is unavailable rather than falling back to driver read mode.

## Observability

Record proof age/failures, query allow/deny counts, latency, queue wait,
truncation, cancellation, pool acquisition and recovery. Include target and
version labels but no Cypher or data values. Log privilege mismatch categories
without exposing usernames or the raw privilege catalog.

## Test and acceptance plan

1. Unit-test privilege classification against missing roles, `PUBLIC` load,
   writes, boosted execution, wildcards, other graphs and unknown future
   actions. Tests must also accept scoped read grants.
2. Test config validation, exact URI/TLS endpoint, secret handling, driver
   pool limits, value codecs, metadata bounds and tool argument rejection.
3. Run a disposable pinned Enterprise fixture with a data graph and a private
   graph. Exercise variable-length traversal, subqueries, filtering, vector
   and full-text `SEARCH`, metadata and citation IDs through MCP.
4. Submit `CREATE`, `MERGE`, `SET`, `DELETE`, schema changes, `LOAD CSV`, a
   remote URL, cross-database `USE`, admin commands and boosted procedures;
   show native denial and unchanged data. Change grants after startup and
   verify recheck fails closed.
5. Measure five-minute and multi-minute reads, deadline cancellation, server
   query termination, pool/permit recovery, concurrency saturation and exact
   response byte enforcement. Record the image digest, server patch, driver
   version, role descriptors and test output before labeling live-qualified.

The [disposable 2026.09.0 Enterprise gate](NEO4J-ACCEPTANCE-2026-10-13.md)
passed on 2026-10-13, including native-denial and cancellation evidence.
Production TLS and five-minute sustained reads remain to be qualified on the
intended deployment endpoint.

## Rollout and operations

Provision the Enterprise role and test user out of band, audit `PUBLIC` and
custom grants, then run the live gate on the exact build. Enable one low-volume
target first. Monitor proof failures, cancellation, pool wait and graph query
latency. Disable the target on any privilege drift or unexpected plugin/role
change; do not relax read-only rules as an incident workaround.

## Alternatives considered

Community Edition users have implied admin authority. Driver read mode routes
requests but is not a security guarantee. A lexical Cypher subset would break
valid GraphRAG reads while failing to prove server-side effects. Broad `reader`
grants cover all databases and do not enforce the configured graph boundary.
The chosen Enterprise proof retains native read syntax under exact grants.

## Completion criteria

The connector is complete only after the registered MCP tool, versioned
authority proof/recheck, native metadata, typed results, resource and
cancellation tests, documentation and the pinned Enterprise live gate all pass.
The local connector and disposable Enterprise gate are implemented and have
passed on 2026.09.0. A different patch or deployment endpoint requires a new
qualification run before claiming those results for it.

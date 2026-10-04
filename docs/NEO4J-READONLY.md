# Neo4j read-only connector

The Neo4j target exposes `neo4j_read` for native Cypher and `neo4j_metadata`
for indexes, constraints, labels and relationship types. It is locally tested,
but **not yet qualified against a live Enterprise server**. The pinned server
patch must be Neo4j Enterprise 2026.03 or newer; the example uses 2026.09.0.
See [RFC-0008](RFC-0008-neo4j-readonly-graphrag.md) for the security model.

## Provisioning contract

Use exactly one configured graph, a dedicated native query user, and a distinct
native attestor user. The query user's only positive grants may be `ACCESS` on
that database, graph `MATCH`/`READ`/`TRAVERSE` there, and `SHOW INDEX` or
`SHOW CONSTRAINT` there. It must not have write, database-management, `LOAD`,
procedure-execution, boosted-execution, wildcard-graph or other-database
grants. This includes grants inherited from `PUBLIC` and other roles. Neo4j's
default `PUBLIC` role grants home-database access and procedure/function
execution; on a disposable instance, revoke every inherited grant that fails
the connector's startup proof. The connector does not alter privileges.

Give the attestor only `SHOW USER`, `SHOW PRIVILEGE`, `SHOW AUTH RULE`, and
`SHOW ROLE` DBMS inspection privileges, with access needed to run these fixed
commands against `system`. `SHOW ROLE` matters: without it, Neo4j may return
`null` for the `roles` column of `SHOW AUTH RULES`; the connector fails closed.
An enabled auth rule with assigned roles blocks this profile, even if its
current condition excludes the query user. This protects against dynamic
role changes between checks. The two accounts use separate secrets.

The query tool accepts a Cypher string and named parameters, but no host,
database, username or URI. The connector executes in a bounded transaction
on that graph. The server grants are the write boundary; Bolt read mode is
only a routing hint. No keyword allowlist is applied to Cypher, so traversal,
subqueries, aggregation, native vector `SEARCH` and native full-text `SEARCH`
remain available when the pinned server supports them. Only query text,
parameters, returned rows and time are bounded. Procedure-based full-text
access is outside the current authority profile.

Start from [the configuration example](../configs/neo4j.example.yaml). Pin
`neo4j.version` to the exact `GetServerInfo` agent patch and set both password
environment variables (or POSIX-protected secret files). Use `bolt+s` with
verified TLS for production; local loopback can use plaintext for a disposable
fixture. The startup proof fails if the server version, user directory,
privilege rows or auth-rule catalog cannot be read in full. It rechecks before
new calls when `privilege_recheck_interval` expires.

## Reproducible Enterprise gate

Provision an isolated Enterprise fixture with a data graph and another private
graph. Create representative linked nodes and a vector index outside the MCP
server. Prepare a config with **exactly one** Neo4j target and both read-only
accounts. Run:

```bash
NEO4J_DISPOSABLE_CONFIG=/absolute/path/to/neo4j-test.yaml \
  go test ./internal/dialects/neo4j -run TestNeo4jDisposableEnterpriseGate -count=1 -v
```

That opt-in test proves startup attestation, a parameterized bounded read,
index metadata, connector write rejection and direct **native** write denial
under the query account. It may create `Neo4jMCPWriteProbe` if the account is
misconfigured, so use a disposable graph.

Then exercise `neo4j_read` through the MCP client with a linked-node traversal,
`CALL {}` read subquery, vector `SEARCH` with a supplied embedding and in-index
filter, and full-text `SEARCH`. Inspect `element_id`, returned scores,
truncation and metadata. Submit `MERGE`, `SET`, `DELETE`, schema changes,
`LOAD CSV`, a cross-graph `USE` and an admin command directly as the query
account; confirm native denial and unchanged data. Revoke the read role and
add a write grant in separate runs; once the proof interval expires, a new
tool call must fail. Exercise a long traversal with a short deadline and
confirm the next request succeeds. Record the server image digest, exact
patch, driver version, roles, test logs, native denials and cancellation
timings before marking this connector live-qualified.

The local test suite cannot establish those server behaviors without a
provisioned Enterprise fixture. The [Neo4j user](https://neo4j.com/docs/operations-manual/current/authentication-authorization/manage-users/),
[privilege](https://neo4j.com/docs/operations-manual/current/authentication-authorization/manage-privileges/),
[auth-rule](https://neo4j.com/docs/operations-manual/current/authentication-authorization/attribute-based-access-control/)
and [vector-search](https://neo4j.com/docs/cypher-manual/current/indexes/semantic-indexes/vector-indexes/)
manuals describe the server features this gate relies on.

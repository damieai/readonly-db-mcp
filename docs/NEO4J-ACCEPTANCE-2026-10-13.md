# Neo4j Enterprise disposable acceptance, 2026-10-13

The Neo4j connector's disposable Enterprise gate passed on a local,
loopback-only Neo4j 2026.09.0 instance. This record covers that exact image
and fixture; it does not establish production TLS behavior or sustained
five-minute query performance.

| Item | Tested value |
| --- | --- |
| Official image | `neo4j:2026.09.0-enterprise` |
| Image digest | `sha256:595846b50b04d5f93a4850f501c6105412dc3ea7b6594126d361720b0f450a52` |
| Go driver | `github.com/neo4j/neo4j-go-driver/v6 v6.2.0` |
| Go toolchain | `go1.26.6 linux/amd64` |
| Transport | Bolt on a dynamically assigned `127.0.0.1` port, plaintext only for this disposable fixture |
| Fixture | `knowledge` and `private` databases; `agent_graph_ro` and `agent_graph_attestor` native accounts; no remaining default `PUBLIC` grants |

The query identity had `PUBLIC` and `agent_graph_reader` roles. The scoped
role's five effective privilege rows were database `ACCESS`, node and
relationship `MATCH`, `SHOW INDEX`, and `SHOW CONSTRAINT`, all for
`knowledge`. The separate attestor role had `SHOW USER`, `SHOW PRIVILEGE`,
`SHOW AUTH RULE`, and `SHOW ROLE` on the DBMS. Credentials were randomly
generated for the disposable instance and are not stored in this repository.

Run with [the example configuration](../examples/neo4j/neo4j-test.yaml)
after provisioning [roles](../examples/neo4j/provision.cypher) and
[data/indexes](../examples/neo4j/fixture.cypher):

```bash
go test -p 1 ./internal/dialects/neo4j ./internal/mcpserver \
  -run TestNeo4jDisposable -count=1 -v -timeout 10m
```

`NEO4J_DISPOSABLE_CONFIG` and the four password/admin variables in the
[connector guide](NEO4J-READONLY.md) must be set for this command. `-p 1`
keeps the MCP and privilege-drift packages from changing the same fixture
concurrently. The [captured test log](#captured-gate-log)
shows all four opt-in tests passing.

The gate verified startup privilege proof; parameterized graph traversal and
source citation; filtered vector and full-text `SEARCH`; index metadata;
result row and byte bounds; and an MCP round trip. Direct calls as the query
identity returned Neo4j `Security.Forbidden` for `CREATE`, `MERGE`, `SET`,
`DELETE`, index creation, local and remote `LOAD CSV`, cross-database `USE`,
procedure execution, and `SHOW USERS`. The test checked that the write probe
did not alter the fixture. Revoking the read role or adding `WRITE` made the
next proof fail closed; restoring the role recovered reads.

The cancellation test observed an active query in `SHOW TRANSACTIONS` before
canceling it, then observed the server transaction disappear. In the captured
run, the client returned in 608 µs and the transaction disappeared in 42 ms;
these are observations, not latency guarantees. A short query deadline and a
one-permit queue saturation check also released their resources and allowed a
subsequent read. The full Go suite, `go vet ./...`, and focused Neo4j race
tests passed on the same code.

Remaining deployment checks are verified TLS against the intended production
endpoint, five-minute and other sustained reads under its resource limits,
and a new authority/hostile-query run after any server or driver upgrade.

On 2026-10-15, a local `bolt+s` driver-handshake test passed for a trusted
CA and matching host, and rejected an untrusted CA and mismatched hostname.
This is a client-side test against a temporary TLS listener, not a Neo4j
server gate. An opt-in five-minute single-read gate and an eight-minute
disposable test configuration are now available. Neither the live TLS gate
nor the sustained-read gate ran on 2026-10-15 because Docker Desktop's WSL
integration was unavailable in the workspace.

## Captured gate log

```text
=== RUN   TestNeo4jDisposableCancellationRecovery
    cancellation_live_test.go:112: client cancellation returned in 608.137µs
    cancellation_live_test.go:117: server transaction disappeared in 42.428043ms
--- PASS: TestNeo4jDisposableCancellationRecovery (2.36s)
=== RUN   TestNeo4jDisposablePrivilegeDrift
--- PASS: TestNeo4jDisposablePrivilegeDrift (4.08s)
=== RUN   TestNeo4jDisposableEnterpriseGate
=== RUN   TestNeo4jDisposableEnterpriseGate/native_denial_create
=== RUN   TestNeo4jDisposableEnterpriseGate/native_denial_merge
=== RUN   TestNeo4jDisposableEnterpriseGate/native_denial_set
=== RUN   TestNeo4jDisposableEnterpriseGate/native_denial_delete
=== RUN   TestNeo4jDisposableEnterpriseGate/native_denial_schema
=== RUN   TestNeo4jDisposableEnterpriseGate/native_denial_load
=== RUN   TestNeo4jDisposableEnterpriseGate/native_denial_remote_load
=== RUN   TestNeo4jDisposableEnterpriseGate/native_denial_cross_database_use
=== RUN   TestNeo4jDisposableEnterpriseGate/native_denial_procedure
=== RUN   TestNeo4jDisposableEnterpriseGate/native_denial_admin
--- PASS: TestNeo4jDisposableEnterpriseGate (0.47s)
    --- PASS: TestNeo4jDisposableEnterpriseGate/native_denial_create (0.01s)
    --- PASS: TestNeo4jDisposableEnterpriseGate/native_denial_merge (0.02s)
    --- PASS: TestNeo4jDisposableEnterpriseGate/native_denial_set (0.01s)
    --- PASS: TestNeo4jDisposableEnterpriseGate/native_denial_delete (0.01s)
    --- PASS: TestNeo4jDisposableEnterpriseGate/native_denial_schema (0.01s)
    --- PASS: TestNeo4jDisposableEnterpriseGate/native_denial_load (0.03s)
    --- PASS: TestNeo4jDisposableEnterpriseGate/native_denial_remote_load (0.05s)
    --- PASS: TestNeo4jDisposableEnterpriseGate/native_denial_cross_database_use (0.01s)
    --- PASS: TestNeo4jDisposableEnterpriseGate/native_denial_procedure (0.02s)
    --- PASS: TestNeo4jDisposableEnterpriseGate/native_denial_admin (0.02s)
PASS
ok  	github.com/your-org/readonly-db-mcp/internal/dialects/neo4j	7.021s
=== RUN   TestNeo4jDisposableMCPRoundTrip
2026/10/13 13:12:43 INFO server connecting
2026/10/13 13:12:43 INFO server session connected session_id=""
2026/10/13 13:12:43 INFO session initialized
2026/10/13 13:12:44 INFO server session disconnected session_id=""
--- PASS: TestNeo4jDisposableMCPRoundTrip (0.10s)
PASS
ok  	github.com/your-org/readonly-db-mcp/internal/mcpserver	0.121s
```

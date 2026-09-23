# Security policy

## Security objective

The server must be unable to persistently modify a configured database through
any public MCP tool, even when a caller supplies hostile SQL, Redis commands or Elasticsearch tool arguments.

This objective depends on a dedicated database identity whose effective grants
are limited to engine-specific read permissions (including attested T-SQL
functions for SQL Server), or on a Redis ACL limited to attested read commands
and `%R~` document-key patterns. Parsing, command classification, tool
annotations and read-only execution modes are additional controls, not
substitutes for database permissions.

## Trust boundaries

Trusted:

- The operator-owned YAML configuration.
- Secret files or environment variables provided to the local process.
- The database administrator who provisions accounts and safe views.
- The compiled server binary, SQL Server parser helper and pinned dependencies.
- Operator-approved signing keys and exact native Redis module artifacts. A
  signed profile asserts reviewed behavior; it cannot sandbox native module code.
  Redis built-in profiles bind the server executable hash and declared build ID;
  the latter is not cryptographic remote artifact attestation.

Untrusted:

- MCP clients and models.
- Every tool argument, including target, SQL, Redis command vectors, parameters
  and purpose.
- Every database value returned to the model. Database text may contain prompt
  injection and must be treated as data rather than instructions.
- MCP client identity metadata, which is self-reported.

## Defense in depth

1. A target is selected only by exact configured alias. Calls cannot supply a
   network address, DSN or credential.
2. Startup reads `SHOW GRANTS` and refuses unknown roles, global SELECT, extra
   privileges and SELECT access outside configured schemas.
3. The MySQL dialect parser accepts only a single SELECT or UNION AST.
4. The AST walk rejects `INTO`, locking reads, user/system variables, advisory
   locks, GTID waits, stored functions, unknown loadable functions, executable
   comments, optimizer hints and access to system schemas.
5. The Go executor starts a transaction with `sql.TxOptions{ReadOnly: true}`
   and exposes no write tool.
6. Context deadlines, MySQL `max_execution_time`, pool limits, semaphores, row
   limits and byte limits reduce resource-exhaustion risk.
7. Audit logs contain a one-way query fingerprint and table names but never raw
   SQL, parameters, returned values, passwords or DSNs.
8. Redis startup expands the live command/subcommand catalog, verifies the
   effective ACL and read-key patterns, requires signed exact-artifact module
   profiles, and resolves effective key access before execution. Redis Search
   additionally verifies canonical indexes and every indexed key prefix. A legacy
   logical-index ACL rule is admitted only for signed read commands and a namespace
   disjoint from document keys; it does not admit data-changing commands.
9. Redis ACL and command capabilities are periodically re-attested; drift marks
   the target unhealthy and new requests fail closed.
10. SQL Server requires the pinned ScriptDom helper, complete catalog visibility,
    transitive module checks and native SHOWPLAN before query execution. Missing
    proof fails closed; there is no lexical-parser fallback. Native transactions
    always roll back, and a connection whose SHOWPLAN cleanup fails is discarded.

11. Elasticsearch exposes fixed metadata and native read routes, verifies TLS and
    pinned cluster/build identity on every configured origin, and proves the
    realm user's effective authority is read-only and within index scope.
    Proof expiry or recheck failure blocks new calls. Wildcard containment is
    checked against future names; metadata resolution and response checks reject
    alias/data-stream scope escapes. Native read and owned-context operations
    remain separately classified from administrative mutations.
    Native date math index selectors are resolved on the pinned endpoint, with
    their resulting sources checked before execution and again before return;
    configuration and role patterns keep ordinary index-glob semantics.
    Alias and settings metadata use scoped native index paths and verify every
    returned index; alias names are checked against the configured scope.
    Query visitors prove embedded lookup/document sources, including base64
    wrapper queries and rendered templates. Stored scripts are frozen to inspected
    inline definitions; only Painless/expression execution and Mustache rendering
    are admitted. Template envelope diagnostic flags follow native defaults and
    precedence; simulation keeps the rendered values. Multi-template reads use
    `es_batch`: every member is rendered/proved before execution, and literal
    Mustache markers in rendered data are never expanded again. Frozen searches
    use the cancellable search/msearch execution routes; the pinned native
    template execution routes do not bind tasks to HTTP channel cancellation.
    Query node inventories must match the pinned stock build without
    external plugins. Implicit semantic-text inference needs its own profile.
12. Elasticsearch ignores environment proxies, node advertisements and HTTP
    redirects. The official transport disables SDK retries and shares a bounded
    HTTP/1.1 socket pool across origins. Raw MCP JSON handling prevents duplicate
    key loss and float conversion of large numbers. Audit fingerprints are keyed
    per process; native error bodies and credentials are not returned or logged.

13. Elasticsearch PIT/scroll/SQL handles are random, process-local and bound to the
    target, transport session, privilege-proof revision and original source scope.
    Native IDs never cross the tool boundary; clear-all is unavailable. Advances
    and closes serialize per handle, tracking the most recent native ID. Authority
    changes include DLS/FLS proof changes, not only new write privileges. Cleanup
    works after a target becomes unhealthy. Failed/ambiguous context requests keep
    their context/memory reservation through cleanup or a conservative lease
    horizon, and never retry creation or advancement. Canceled callers release
    query admission before bounded maintenance recovery. Idle contexts hold no
    connection or execution permit. Disconnect, expiry and shutdown close owned
    contexts; `es_cursor` is read-only but deliberately not marked idempotent.

14. Synchronous EQL uses the complete version-pinned upstream grammar, compiled
    into Go. It consumes one statement through EOF and preserves the original
    query text. Event categories, field names and literals cannot choose indices;
    REST scope and native filter/runtime lookups are proved independently. The
    parser has per-request DFA caches, byte/token/depth/work/cancellation checks
    and separate process memory admission. Parser errors omit query excerpts.
    Non-negative async wait controls and retained results are unavailable. Native
    shard/timeout/partial failures return no data; oversized sequences are never
    cut apart. Unexpected async IDs/running responses invalidate the target's
    profile proof. Native function and pipe semantic checks stay with ES.

15. Native SQL uses a distinct complete upstream grammar for each pinned version.
    Structured visitors cover relation nodes, CTE definitions/references, nested
    queries, joins and metadata patterns (including bound pattern parameters).
    Local catalogs, request index scope, filters, runtime lookups and full-text
    inference effects are proved before execution. Native SQL text/parameters
    are preserved; function and relational semantics remain native decisions.
    SQL query pages have owned transient tokens, and stateless queries drain
    them under one deadline and total result limit. Public SQL cursor handles
    use the same session/authority binding, capacity retention and recovery as
    PIT/scroll. Native tokens, async IDs, partial data and truncated rows are not
    returned. Row/columnar orientation and dimensions are checked on every page.

16. Synchronous ES|QL uses the complete grammar and imports at each fixed commit.
    Release-only lexer/parser predicates, native parameter binding and EOF are
    preserved. Structured visitors cover FROM, JOIN, nested EXPLAIN/FORK and
    quoted comma-separated sources. Exclusions retain their original semantics
    while proof conservatively covers every positive source. Filters/runtime
    lookups use the existing DSL visitors. Full-text checks follow field aliases,
    assignments and renames, including parameterized function/field identifiers;
    ambiguous pattern lineage requires mapping-wide inference proof.
    The fixed `/_query` route cannot select async execution. Partial responses,
    unexpected async/cursor state, shape mismatches and resource overflows fail
    whole. HTTP cancellation shares the request deadline; native parameters and
    exact JSON numbers remain intact. ES|QL shares parser memory admission with
    SQL/EQL, with an 8 MiB baseline for its larger grammar.

17. ENRICH is default-off and requires `enrich.scope=all_cluster_snapshots` plus
    explicit read-only `monitor_enrich`. This authorizes every historical/current/
    future enrichment snapshot in the pinned local cluster, independently of
    ordinary index allow/deny patterns and source DLS/FLS. It does not offer
    per-policy/document/field isolation; use a separate cluster if needed. All
    current snapshot mappings, write blocks, identities and active aliases are
    checked, including retired snapshots. Policy and inventory checks detect
    observed drift and reject the whole result. They cannot fence stale distributed
    state or atomically lock administrator actions; the broad explicit data grant
    and controlled native provisioning are the authorization contract. Normal
    FROM/JOIN scope still applies. Policy create/execute/delete and `manage_enrich`
    remain forbidden; fixed metadata proof routes are internal only.

## Known limitations

- Elasticsearch native queries have TLS fixture coverage, not live engine certification.
  API-key, custom plugin/inference, finer ENRICH isolation, experimental ES|QL pragmas,
  persisted async language results, dynamic suggestion
  collate and cross-cluster profiles are not yet implemented. Pinned version/hash metadata is an operator trust
  check, not cryptographic remote binary attestation.
- ES query preflight is not an atomic lock on administrator changes to mappings,
  aliases, scripts or node membership. Native scoped grants remain the final index
  authorization boundary. HTTP cancellation has fixture coverage; actual ES task
  termination, privilege actions and licensed features require live qualification.
- A valid SELECT can still consume database CPU before timeout or read sensitive
  data the account is allowed to see.
- A valid Redis read can still consume server CPU beyond the client timeout.
  Admission limits reduce amplification; exploratory keyspace-wide reads should
  use a replica where possible.
- Output-name-based masking cannot safely prevent transformed-column exfiltration.
  Use column grants or curated views for confidentiality boundaries.
- A replica can return stale data. Every response reports the configured
  consistency classification, but the server cannot prove replica freshness.
- `tls.mode: required` encrypts without authenticating the server and is refused
  for production. Use `verify-full` with a trusted CA.

## Secrets

- Never commit passwords, certificates, private keys or populated local config.
- Prefer a mounted secret file with mode `0600`, a dedicated environment
  variable, or a secret-agent-managed file.
- Never place secrets in command arguments: process listings and client logs may
  record them.
- Rotate a credential immediately if it reaches model context, chat history,
  source control, CI logs or an issue tracker.

## Reporting vulnerabilities

Do not open a public issue for a suspected vulnerability. Contact the repository
owner privately and include the affected version, reproduction steps and impact.

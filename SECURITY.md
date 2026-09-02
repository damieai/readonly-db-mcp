# Security policy

## Security objective

The server must be unable to persistently modify a configured database through
any public MCP tool, even when a caller supplies hostile SQL or Redis commands.

This objective depends on a dedicated database identity whose effective grants
are limited to `USAGE` and `SELECT`, or on a Redis ACL limited to attested read
commands and `%R~` key patterns. Parsing, command classification, tool
annotations and read-only execution modes are additional controls, not
substitutes for database permissions.

## Trust boundaries

Trusted:

- The operator-owned YAML configuration.
- Secret files or environment variables provided to the local process.
- The database administrator who provisions accounts and safe views.
- The compiled server binary and pinned dependencies.

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
   effective ACL and read-key patterns, rejects modules, and resolves effective
   key access before execution.
9. Redis ACL and command capabilities are periodically re-attested; drift marks
   the target unhealthy and new requests fail closed.

## Known limitations

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

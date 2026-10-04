# SQLite read-only connector, P3

SQLite targets use the common `schema_list_tables`, `schema_describe_table`,
`query_select`, `query_explain` and `query_batch` tools. Copy
[the example configuration](../configs/sqlite.example.yaml) and set `sqlite.root`
and `sqlite.file` to an existing directory and an existing regular database
file. Relative paths are resolved from the configuration file. No host,
credentials, TLS or `ATTACH` path can be supplied by a tool call.
The pinned `go-sqlite3` driver requires CGO and a C compiler when building the
server.

The connector resolves the configured file beneath the configured root at
startup and before each operation, rejecting symlink escapes and replacement
of the opened file's identity. Restart the target after an atomic file swap. SQLite opens
the file with `mode=ro` and `query_only=1`; each pooled connection verifies
`query_only`, turns off `trusted_schema`, sets SQL/value/variable limits and
installs a native authorizer. The authorizer permits SELECT, reads from the
main database, ordinary built-in functions and rollback-only transaction
control. It denies writes, schema changes, `ATTACH`, `DETACH`, extension/file
functions and arbitrary PRAGMAs. The tool also rejects multiple statements.
Do not set `immutable=1` for a database that another process can update:
[SQLite documents](https://www.sqlite.org/uri.html) that it skips locking and
change detection and can return wrong results if the file changes.

Advanced read SQL remains available: joins, CTEs (including recursive CTEs),
window functions, JSON and FTS tables can be queried. Use `?` placeholders for
values. BLOB results are base64 strings; integers outside JSON's safe integer
range are decimal strings. Single queries and batches have row, cell, result
byte and deadline bounds; batches execute in one SQLite snapshot and roll back
after reading. Metadata is limited to the configured file's `main` schema.

For a reproducible local acceptance run, execute:

```sh
go test -race ./internal/dialects/sqlite ./internal/config
```

The SQLite tests create a disposable database, exercise joins/CTEs, JSON, FTS,
metadata, explanation, batching and cancellation, then verify that native
`UPDATE`/`ATTACH`, tool-level writes, extension loading, multi-statements and
symlink escapes fail. No external database service is needed. They cover the
bundled SQLite build in the pinned Go driver; the exact version can be queried
with `SELECT sqlite_version()` on a configured target.

The directory containing a SQLite database should be controlled by the
operator. If another process writes the database in WAL mode, keep its `-wal`
and `-shm` files available with the database file; SQLite may need them for a
read-only open. This connector has no writer/checkpointer API.

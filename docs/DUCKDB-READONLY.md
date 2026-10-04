# DuckDB read-only connector

DuckDB targets use the common `schema_list_tables`, `schema_describe_table`,
`query_select`, `query_explain` and `query_batch` tools. They support ordinary
analytical SELECT queries, joins, CTEs, windows, JSON, parameter binding and
local Parquet reads. Copy [the example configuration](../configs/duckdb.example.yaml)
and set `duckdb.root` to a dedicated operator-controlled directory containing
the existing database file and approved Parquet or other local data files.
The root may not be `/`. The entire root is visible read-only inside the worker
as `/data`; use `/data/<relative-file>` in file-reading SQL functions.

Build the separate executable with `make build-duckdb-worker` and set
`duckdb.worker_path` to its absolute path. Linux and an installed `bwrap`
(bubblewrap) executable are required. The connector pins DuckDB `1.5.6` from
the `duckdb-go/v2` `v2.10506.0` bundle; set `duckdb.version: 1.5.6`.
`memory_mb` and `threads` control each worker; `per_target_concurrency`
controls the maximum concurrent workers for that target. Provision enough
host memory for their combined budget. Startup launches an actual isolated
worker and refuses to register the target if the sandbox, version or read-only
settings cannot be proved.

Every request starts a fresh worker in bubblewrap with network isolation, a
read-only mount of the data directory and no writable view of the host. It
opens the configured file with native `READ_ONLY` access and locks DuckDB
configuration after disabling automatic extension installation/loading,
unsigned extensions and external file access. Only the approved `/data`
directory remains readable to SQL. A single native SELECT statement is
required; a lexical multi-statement guard runs before driver preparation
because the driver otherwise executes preceding statements while preparing.
The SQL shape is not reduced to a small allowlist of analytic constructs.
[DuckDB's own security guidance](https://duckdb.org/docs/current/operations_manual/securing_duckdb/overview)
recommends process isolation for untrusted SQL; the worker provides that
boundary and is mandatory for this connector.

The database file and worker executable are pinned by resolved path and file
identity for the target lifetime. Restart after replacing either file. Place
the data directory and worker outside directories writable by MCP clients.
The worker can create temporary files in an isolated, disposable `/tmp`; it
cannot persist them to the host. Queries have deadlines and row, cell, result,
SQL and parameter byte limits. Batches run in a single DuckDB snapshot and
roll back. Binary values are base64 strings; integers outside JSON's safe
integer range are decimal strings. The common schema tool reports columns and
index names.

For a reproducible local acceptance run:

```sh
make test-duckdb-local
```

The test creates a disposable DuckDB database and Parquet file. It verifies
advanced SELECT, parameters, metadata, EXPLAIN, batching, native mutation
denial, multi-statement denial, file and network isolation, query cancellation
and recovery. No database service or Docker container is needed. If the host
cannot create bubblewrap namespaces, startup fails closed and this live test
cannot pass; grant the process the OS namespace capability in its deployment
environment before enabling a target.

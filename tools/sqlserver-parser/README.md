# SQL Server parser helper

The Go adapter requires this helper at startup. It uses Microsoft ScriptDom
170.191.0 with parsers for compatibility levels 150, 160 and 170. The original
SQL is executed only after AST/source analysis, live module closure checks and
SQL Server SHOWPLAN validation. There is no production scanner fallback.

Build with the .NET 8 SDK:

```bash
make build-sqlserver-parser
make test-sqlserver-parser
make build
```

The self-contained output is `bin/sqlserver-parser/`. Ship that entire directory
beside `bin/readonly-db-mcp`; the deployment does not need an installed .NET
runtime. Override `DOTNET` or `SQLSERVER_RUNTIME` for a different SDK location
or a separately qualified platform. `packages.lock.json` pins the parser
package and integrity hash; .NET runtime 8.0.31 is pinned in the project.
Runtime upgrades require another corpus and packaging run.

Alternatively set `sqlserver.parser_path` to the absolute executable path (or
a path relative to the configuration file). This is operator configuration,
never a tool argument. The executable receives no database credentials or
inherited environment. Standard error is discarded to avoid query disclosure.

Each process handles one big-endian 32-bit length-prefixed JSON request and
response, capped at 1 MiB, then exits. The request contains `SQL`,
`Compatibility` and `Module`; the response contains protocol version, acceptance,
source references, parameters and node count. AST depth is capped at 256 and
node count at 100,000. Unknown source-bearing table nodes fail closed. All
children of the pinned AST are traversed, including nested function bodies and
branches. Errors never contain SQL source. The Go parent validates framing,
response shape, source kinds and parameter identities independently.

Helper processes are launched inside the request's existing admission permit
and deadline. Cancellation kills the child and reaps it. One-shot execution
avoids shared parser state and restart loops, at the cost of per-call startup;
a future pool requires latency/memory evidence before adoption. The CLR heap
ceiling is 256 MiB per active helper, in addition to native/runtime overhead.
Capacity planning must include that separate process memory; the existing Go
response/cache forecast does not include it.

The corpus exercises advanced reads and hostile statements against all three
parser compatibility levels. This is not a substitute for running the real
SQL Server version, permission, cancellation and driver matrices.

# Contributing

Changes that broaden accepted SQL, database privileges, network transports,
secret sources or result sizes are security changes and require explicit threat
model review.

RFCs must start from [the RFC template](docs/RFC-TEMPLATE.md). Any RFC or pull
request that adds an infrastructure engine, changes a driver, or changes
timeouts, admission, batching, response limits, or connection pools must include
a latency and pool budget review against the
[agent workload contract](docs/ARCHITECTURE.md#agent-workload-latency-budget).

The review must:

- list every effective request, queue, phase, driver, server, cleanup, and pool
  timeout, including defaults and hard ceilings;
- state whether one deadline covers the whole batch or each operation;
- preserve the 5-minute default and 10-minute configurable request budget, or
  justify a tighter limit with measurements and an operator override;
- prove that no driver or server timeout silently shortens the advertised
  request budget;
- verify that configured connection limits map to real driver hard limits;
- test saturation without sub-second accidental rejection, cancellation of
  server work, and prompt permit and connection reuse; and
- update `internal/config`, `configs/example.yaml`, tests, and the relevant
  architecture or operational documentation together.

Before submitting a change:

```bash
make fmt
make test-race
make vet
make build
```

When adding accepted SQL syntax, include both an allowed example and nearby
hostile variants that must remain rejected. When adding a function, determine
whether MySQL represents it as a built-in AST node, generic function, stored
function or loadable function, and document why it has no side effects.

Do not include real database names, hosts, credentials, customer identifiers or
query results in tests, examples, commits or issues.

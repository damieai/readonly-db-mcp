# Contributing

Changes that broaden accepted SQL, database privileges, network transports,
secret sources or result sizes are security changes and require explicit threat
model review.

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

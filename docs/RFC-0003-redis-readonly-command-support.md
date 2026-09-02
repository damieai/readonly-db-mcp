# RFC-0003: Redis Read-Only Command Support

- Status: Accepted; standalone phases 1-3 implemented
- Authors: readonly-db-mcp maintainers
- Created: 2026-09-02
- Target release: staged
- Scope: Redis targets over the existing stdio MCP transport

Implementation note: the current code implements standalone Redis, the common
capability split, command/ACL attestation, key-scope validation, RESP3 result
handling, read-only scripting, pipelines and atomic batches. Sentinel, Cluster
and attested third-party modules remain gated rollout phases; configuration
fails closed instead of silently enabling those modes.

## Summary

This RFC adds Redis as a first-class target without pretending that Redis is a
SQL database. Redis receives command-vector MCP tools, a Redis-specific target
interface, server command-catalog attestation, ACL attestation, key-scope
validation and bounded RESP result normalization.

The policy is **deny by persistent or externally observable side effect, not by
command sophistication**. Advanced read operations remain available when Redis
can prove that every accessed key is read-only and within the configured target:
multi-key reads, sorted-set algebra, streams, geospatial queries, probabilistic
structures, cursor scans, read-only Lua scripts, read-only Functions and
properly declared read-only module commands are not rejected merely because
they are uncommon or expensive.

Commands that can create, overwrite, update, expire, rename, move or delete
data are rejected. Commands that mutate consumer-group state, server/session
configuration, cluster topology, subscriptions, transactions outside the
managed batch path, locks or other externally observable control-plane state
are also rejected. Target/key boundaries and resource ceilings remain security
and availability constraints rather than restrictions on Redis expressiveness.

The Redis ACL user is the final data-integrity boundary. Application command
classification, key extraction and optional replica routing are defense in
depth. The service refuses startup when the effective ACL or server command
catalog cannot be proven read-only.

## Motivation

The project already provides read-only access to relational systems, but Redis
requires a different model:

- Redis accepts command vectors rather than a query language;
- command arguments may dynamically identify one or many keys;
- some commands are read-only only in a dedicated variant, such as `SORT_RO`,
  `BITFIELD_RO`, `EVAL_RO` and `FCALL_RO`;
- apparently observational commands can modify TTLs, stream consumer state,
  connection state or the server control plane;
- modules can add commands whose safety metadata is version-dependent;
- cluster routing and multi-key hash-slot rules affect execution;
- RESP replies are heterogeneous, nested and potentially very large.

A static list copied from one Redis release would become stale and would either
block legitimate advanced reads or accidentally admit a newly changed command.
The design therefore combines a small invariant deny set with the command
metadata reported by the exact server being used.

## Goals

- Support Redis through dedicated MCP command tools and exact target aliases.
- Reject every command invocation that may modify stored key values, key
  metadata, TTLs, stream consumer state or control-plane state.
- Permit advanced reads based on proven side-effect semantics rather than a
  narrow list of common commands.
- Support Redis key types including strings, hashes, lists, sets, sorted sets,
  streams, bitmaps, HyperLogLog reads and geospatial reads.
- Support cursor scans and complex multi-key read commands.
- Support `EVAL_RO`, `EVALSHA_RO` and `FCALL_RO` with server-enforced no-write
  semantics and explicit resource limits.
- Admit module commands only when their installed implementation and command
  metadata have passed a configured attestation policy.
- Enforce configured key patterns independently of the Redis ACL.
- Reuse admission control, timeouts, exact response-byte budgets, metrics,
  credential handling, TLS and structured audit events.
- Support standalone, Sentinel and Redis Cluster deployments in staged phases.
- Re-attest ACLs and command metadata periodically and fail closed on drift.

## Non-goals

- Exposing arbitrary raw RESP bytes or a general-purpose Redis console.
- Allowing callers to select a database, endpoint, username or credentials.
- Supporting write commands even when their particular arguments appear to be
  harmless, idempotent or directed at a nonexistent key.
- Treating `GETEX`, `XREADGROUP`, `SORT ... STORE`, `BITFIELD SET/INCRBY`,
  `MIGRATE`, `COPY`, `RESTORE`, `PUBLISH` or lock commands as reads.
- Providing Redis administration, replication, backup, failover or cluster
  management.
- Exposing Pub/Sub or `MONITOR` through request/response MCP tools.
- Loading scripts or Functions. Read-only invocation of already supplied code
  is separate from lifecycle management.
- Trusting an unverified third-party module merely because it labels a command
  read-only.
- Guaranteeing confidentiality for keys the configured ACL may legitimately
  read. Separate targets and key prefixes remain the confidentiality boundary.
- Providing a consistent cross-command snapshot for a non-atomic pipeline.
- Supporting Valkey or Redis-compatible products until each product has its own
  tested command-metadata and ACL compatibility matrix.

## Supported versions

The first production matrix targets Redis Open Source 7.2 and explicitly tested
Redis 8.x minors. Version admission is exact and configurable in code; an
unknown major or untested command-catalog format fails startup.

Redis 7.2 is the minimum because the design relies on:

- ACL users and effective command rules;
- read-key ACL patterns such as `%R~prefix:*`;
- modern command key specifications;
- `COMMAND GETKEYSANDFLAGS`-equivalent key extraction;
- read-only scripting variants.

Redis Cloud and Redis Software require their own integration profiles because
their ACL and module behavior may not be identical to Redis Open Source.

## Security model

### Policy principle

An invocation is admitted only when all of the following are proven:

1. The command exists in the startup-attested catalog.
2. Its effective command flags classify it as read-only.
3. Every effective key specification is `RO`, never `RW`, `OW` or `RM`.
4. The invocation contains no option that selects a write path.
5. Every resolved key is within one configured read-key pattern.
6. The effective Redis ACL independently permits only the same read-only
   command and key scope.
7. It does not mutate session, transaction, subscription, consumer-group,
   cluster, replication, persistence, scripting-library or server state.
8. It stays within configured request, blocking, reply and concurrency limits.

Complexity is not a rejection reason by itself. For example, `ZINTER`,
`ZUNION`, `GEOSEARCH`, `XREAD`, `XRANGE`, `SCAN`, `HSCAN`, `SSCAN`, `ZSCAN`,
`SORT_RO`, `BITFIELD_RO` and a nested `EVAL_RO` program remain admissible when
the above proof succeeds.

### Final boundary: dedicated ACL user

Every Redis target uses a dedicated ACL user created from a reset state. For
Redis Open Source 7.2+, its intended shape is conceptually:

```text
ACL SETUSER readonly_mcp reset on >generated-secret \
  -@all +@read \
  +command +command|info +command|list +command|getkeysandflags \
  +acl|whoami +acl|getuser +ping +info +module|list \
  +eval_ro +evalsha_ro +fcall_ro \
  %R~analytics:* resetchannels
```

This is illustrative, not a copy-paste universal policy. Startup expands every
granted category against the current server catalog and rejects unsafe members.
Production provisioning should prefer an explicit generated command list whose
hash is recorded alongside the deployment.

The user must have:

- no write-key pattern (`%W~`, `%RW~`, `~` or `allkeys`);
- one or more `%R~` patterns exactly covered by the configured target patterns;
- no channel patterns;
- no `+@all`, `+@write`, `+@admin`, `+@dangerous`, or unclassified category;
- only individually attested connection/introspection commands beyond reads;
- no selector that broadens commands, keys or channels;
- no permission to load code, modules or Functions;
- no permission to change ACLs, configuration, persistence or topology.

`ACL GETUSER` is allowed only so the service can attest its own effective ACL.
It is never exposed as a caller-selectable Redis command. If a managed Redis
provider cannot grant safe self-introspection, configuration must supply an
out-of-band signed ACL attestation generated by a privileged deployment check;
silently skipping ACL verification is not allowed.

### Defense in depth

1. Exact configured target lookup; callers cannot supply an address or DSN.
2. TLS and authenticated Redis username/password.
3. Startup server-version, identity, ACL and command-catalog attestation.
4. Command-vector validation; no shell-like tokenization or raw protocol input.
5. Server-derived key extraction followed by local key-pattern authorization.
6. Command flag, key-spec and option-sensitive side-effect classification.
7. Dedicated read-key-only ACL enforcement by Redis.
8. Optional reads from a replica configured by operators as read-only.
9. Bounded admission, blocking time, script size, nesting, element count and
   exact encoded response bytes.
10. Periodic ACL/catalog re-attestation with fail-closed health state.
11. Query-content-free audit events and low-cardinality metrics.

### Why ACL categories alone are insufficient

`+@read` is useful but is not the entire proof:

- category membership changes when Redis is upgraded;
- module commands historically are not necessarily included in `@read`;
- key patterns do not constrain keyless commands such as `FLUSHALL`;
- commands such as `SORT` combine read and optional storage behavior;
- server and connection commands can have no key while still changing state.

The service expands categories, inspects the exact server's `COMMAND INFO`
metadata, verifies key specifications, and applies invariant denials.

### Invariant forbidden effects

The policy rejects commands or subcommands that can:

- insert, update, overwrite, rename, move, expire, persist or delete a key;
- change stream groups, consumers, pending entries or acknowledgement state;
- publish messages or enter a subscription mode;
- acquire distributed locks or write advisory state;
- change the selected database, authentication, client name, tracking mode or
  other caller-controlled persistent connection behavior;
- enter caller-managed `MULTI`, `WATCH`, `EXEC` or `DISCARD` state;
- change ACLs, configuration, modules, Functions, scripts, persistence,
  replication, Sentinel or cluster topology;
- trigger failover, shutdown, migration, restore, swap or flush behavior;
- expose cross-target traffic or secrets through `MONITOR`, tracing or broad
  client inspection;
- execute an unclassified command or unresolved key specification.

The managed batch implementation may internally use `MULTI`/`EXEC`, and the
cluster transport may internally issue `READONLY`. These internal capabilities
are not caller-selectable commands.

## Command classification

### Startup capability catalog

At startup the Redis target obtains and canonicalizes:

- `HELLO 3` identity and protocol information;
- server version and deployment mode;
- `ACL WHOAMI` and the effective `ACL GETUSER` response;
- `COMMAND INFO` for all commands and subcommands;
- command flags, arity, ACL categories, tips and key specifications;
- installed module list and module-provided command ownership;
- replication role and cluster state when configured.

The canonical catalog is hashed. The hash is included in target diagnostics and
audit startup events but not exposed with credentials or ACL password hashes.

A command is automatically eligible only if:

- it has the `readonly` flag and not `write`;
- every key specification is `RO` and contains no write logical flag;
- it has no incomplete, unknown or variable write specification;
- its command/subcommand is not in the invariant side-effect set;
- its installed implementation belongs to the tested Redis core catalog, or it
  satisfies the module attestation rules below.

The final allowed set is the intersection of the classified catalog, configured
policy and effective ACL. It is never the union.

### Option-sensitive commands

Prefer Redis-provided read-only variants rather than locally proving a safe
subset of a write-capable command:

| Reject | Admit equivalent |
| --- | --- |
| `SORT` | `SORT_RO` |
| `BITFIELD ... GET ...` | `BITFIELD_RO ... GET ...` |
| `EVAL` / `EVALSHA` | `EVAL_RO` / `EVALSHA_RO` |
| `FCALL` | `FCALL_RO` for a `no-writes` Function |
| `GETEX` | `GET` plus a separate `TTL`/`PTTL` read |
| `XREADGROUP` | `XREAD`, `XRANGE` or `XREVRANGE` |

This keeps advanced functionality while asking Redis itself to enforce the
read-only execution path.

### Read-only programmability

`EVAL_RO`, `EVALSHA_RO` and `FCALL_RO` are supported because Redis rejects
write commands from these execution modes. They still receive stricter resource
governance because a script runs synchronously and may block the server:

- maximum script bytes;
- maximum `KEYS` count and argument bytes;
- configured maximum execution timeout;
- separate script admission class and concurrency cap;
- result depth, element and byte limits;
- no automatic retry after an ambiguous timeout;
- caller-declared keys must all pass local key-pattern validation;
- `FCALL_RO` requires an attested `no-writes` function from an allowed library
  catalog hash.
- Prefix-scoped targets reject programmability in the standalone first release:
  a script can invoke keyless `SCAN`/`KEYS` internally, which Redis ACL key
  patterns cannot constrain. Read-only scripts currently require a target whose
  exact ACL and configuration both use `%R~*`; a future separate script ACL may
  restore prefix-scoped programmability without weakening isolation.

The service never uses `SCRIPT LOAD`, `FUNCTION LOAD`, `FUNCTION RESTORE`,
`FUNCTION DELETE` or `FUNCTION FLUSH`.

### Modules

Redis modules execute native code and are capable of reporting incorrect or
incomplete command metadata. Therefore module support is not enabled merely by
seeing a `readonly` flag.

A module command is eligible only when:

- the module name and exact version/build hash appear in `allowed_modules`;
- its commands have a reviewed snapshot of flags and key specifications;
- the live catalog exactly matches that snapshot;
- integration tests prove write attempts fail under the target ACL;
- all key specs are complete and read-only;
- the command has no external side effect outside Redis key data.

Unknown or changed module builds fail closed. This policy can support advanced
Redis Search/JSON/time-series reads without trusting every installed module.

## Key extraction and authorization

### Key extraction

The service does not guess key positions from command names. It uses the
attested command key specifications and, where supported, asks the same Redis
node for `COMMAND GETKEYSANDFLAGS` before execution.

The preflight returns the effective keys and their access flags for the exact
argument vector. The service rejects the invocation when:

- extraction fails or is ambiguous;
- a returned key is `RW`, `OW` or `RM`;
- an unknown or incomplete specification remains;
- a key is outside all configured target patterns;
- the number or total bytes of keys exceeds configured ceilings.

The Redis ACL independently repeats key authorization at execution time.

### Key patterns

Configuration uses Redis glob patterns but accepts only a reviewable subset by
default: literal prefixes ending in `*`, such as `analytics:*`. Character
classes, single-character wildcards and broad `*` require an explicit
`allow_complex_key_patterns` acknowledgment because pattern containment is
otherwise difficult to prove.

The application patterns must be equal to or narrower than every effective
`%R~` ACL pattern. A broader ACL than configuration fails startup instead of
relying only on application validation.

### Keyless reads

Keyless commands are not automatically safe. A small semantic class admits
observational commands needed for health and ordinary operation, such as
`PING`, `ECHO`, `TIME`, `DBSIZE` and bounded server information sections.

Control-plane, cross-client and secret-bearing observations remain internal or
forbidden even if Redis labels them read-only. In particular, callers cannot
invoke `ACL`, `COMMAND`, `CONFIG`, `CLIENT`, `CLUSTER`, `SENTINEL`, `MODULE`,
`FUNCTION`, `SCRIPT`, `MONITOR`, `ROLE`, replication or persistence commands.

## MCP API

Redis uses dedicated tools rather than overloading SQL fields.

### `redis_command`

Input:

```json
{
  "target": "analytics-redis",
  "command": "ZINTER",
  "arguments": [
    "2",
    "analytics:score:daily",
    "analytics:score:verified",
    "WITHSCORES"
  ],
  "timeout_ms": 3000,
  "max_elements": 1000,
  "purpose": "compare verified daily scores"
}
```

The command is a single name. Arguments are already separated; no quoting,
escaping, comments, pipes or shell parsing exists. Command names and
subcommands are case-insensitive but canonicalized for policy and audit.

Binary arguments use an explicit wrapper rather than ambiguous JSON strings:

```json
{"base64": "AAECAwQ="}
```

Ordinary strings and lossless integers are also accepted. Floating-point
values are serialized canonically; NaN and infinities are rejected.

Output:

```json
{
  "request_id": "...",
  "target": "analytics-redis",
  "engine": "redis",
  "command": "ZINTER",
  "value": ["member-a", "42.5"],
  "element_count": 2,
  "truncated": false,
  "duration_ms": 3
}
```

### `redis_batch`

Executes several validated read-only command vectors.

- `atomic: false` uses a bounded pipeline and preserves input order.
- `atomic: true` uses an internally managed `MULTI`/`EXEC` on one connection.
- In Cluster mode, atomic batches require all keys to share a hash slot.
- Callers cannot inject transaction-control commands into the batch.
- The entire encoded batch response shares one exact byte budget.
- A non-atomic batch is not presented as a consistent snapshot.

### Optional convenience tools

`redis_scan` and `redis_key_info` may be added as typed convenience wrappers,
but they call the same policy path as `redis_command`. They are not separate
privilege paths.

SQL-only tools reject Redis targets with an engine-specific error. Redis tools
reject SQL targets. `list_targets` and `inspect_target` remain shared.

## Core interface changes

The current `core.Target` assumes tables and SQL. It becomes a minimal common
target plus capability interfaces:

```go
type Target interface {
    Info() TargetInfo
    Close() error
}

type SQLTarget interface {
    Target
    ValidateQuery(string) (*Validation, error)
    Query(context.Context, QueryRequest) (*QueryResult, error)
    Explain(context.Context, QueryRequest) (*QueryResult, error)
    ListTables(context.Context, string, bool) ([]TableSummary, error)
    DescribeTable(context.Context, string, string, bool) (*TableDescription, error)
}

type RedisTarget interface {
    Target
    ValidateRedis(context.Context, RedisRequest) (*RedisValidation, error)
    RedisCommand(context.Context, RedisRequest) (*RedisResult, error)
    RedisBatch(context.Context, RedisBatchRequest) (*RedisBatchResult, error)
}
```

The MCP layer resolves the target and then requires the appropriate capability.
This avoids dummy table/explain implementations and leaves room for future
non-SQL read-only stores.

`TargetInfo` gains optional Redis fields:

- deployment mode (`standalone`, `sentinel`, `cluster`);
- Redis version;
- selected database where applicable;
- configured key patterns;
- replica-read preference;
- command-catalog revision;
- last successful ACL attestation time.

## Configuration

Example standalone target:

```yaml
targets:
  analytics-redis:
    engine: redis
    environment: production
    consistency: current
    host: redis.internal.example
    port: 6379
    username: readonly_mcp
    password_file: /run/secrets/analytics-redis.password
    redis:
      mode: standalone
      database: 0
      key_patterns:
        - "analytics:*"
      prefer_replica: false
      require_replica: false
      protocol: 3
      acl_recheck_interval: 5m
      command_catalog_recheck_interval: 5m
      command_catalog_max_age: 10m
      allow_readonly_scripts: false
      max_script_bytes: 65536
      max_keys_per_command: 256
      max_argument_bytes: 262144
      max_reply_depth: 32
      max_reply_elements: 10000
      allowed_modules: []
    connection:
      connect_timeout: 3s
      read_timeout: 5s
      write_timeout: 3s
      max_open: 4
      max_idle: 2
      max_lifetime: 3m
      max_idle_time: 1m
    tls:
      mode: verify-full
      ca_file: /etc/readonly-db-mcp/redis-ca.pem
      server_name: redis.internal.example
```

Sentinel adds service name and separately sourced Sentinel credentials. Cluster
mode uses a configured seed list; callers can never provide or override nodes.

Secrets remain restricted to password files or environment variables. URLs and
inline passwords are rejected.

## Deployment modes

### Standalone

One bounded pool connects to the configured endpoint and fixed database. The
service selects the database only during connection initialization. Caller
`SELECT` is forbidden.

### Sentinel

Sentinel discovers the configured service. Discovery endpoints and credentials
are configuration-only. Each newly discovered data node repeats TLS, identity,
role, ACL and command-catalog checks before it becomes usable.

Read preference may use replicas, but failover never silently changes the target
from `require_replica: true` to a primary.

### Cluster

The client routes commands using extracted key slots. Multi-key commands retain
all Redis functionality that the cluster itself accepts; cross-slot failures are
returned as sanitized errors rather than rewritten into multiple operations.

Replica reads use internal `READONLY` connections. The client handles `MOVED`
and `ASK` redirects only to nodes admitted by the cluster topology obtained from
configured seeds. Redirects to arbitrary hosts are rejected.

## Driver decision

Use `github.com/redis/go-redis/v9`, pinned to an audited version.

Reasons:

- RESP2/RESP3 support and binary-safe argument vectors;
- standalone, Sentinel and Cluster clients;
- context-aware calls and bounded pools;
- cluster slot routing and replica-read support;
- generic `Do` support without maintaining typed wrappers for every advanced
  read command.

The generic `Do` method is reachable only after policy validation. Driver
hooks record timings and audit metadata but never raw argument values.

## Execution flow

For each request:

1. Resolve an exact Redis target.
2. Normalize and bound the command vector.
3. Look up the immutable command capability from the current catalog revision.
4. Reject invariant side effects and non-read-only key specifications.
5. Extract effective keys for the exact arguments.
6. Validate every key against target patterns.
7. Acquire the appropriate admission permit.
8. Execute exactly once with the request deadline.
9. Normalize RESP2/RESP3 recursively with depth, element, cell and byte limits.
10. Enforce the exact final JSON response budget.
11. Emit fingerprint-only audit and phase metrics.

Commands are not automatically retried after a timeout or transport error. A
read command should be side-effect-free, but automatic retry could duplicate
expensive work and amplify load.

## Result normalization

RESP values map to JSON as follows:

| RESP | JSON representation |
| --- | --- |
| null | `null` |
| integer | number when JSON-safe, otherwise decimal string |
| double | finite JSON number |
| simple/blob string | UTF-8 string or `{ "base64": "..." }` |
| array/set/push | array; push replies are not admitted on ordinary tools |
| map/attribute | object only for unique UTF-8 keys, otherwise pair array |
| boolean | boolean |
| error | sanitized Redis error class without server text leakage |

Truncation is structural and explicit. The implementation never slice-truncates
UTF-8 or binary data into an ambiguous value. Oversized scalar values use the
existing cell limit and report truncation metadata.

## Resource governance

Redis command timeouts mainly bound the client wait; they do not guarantee that
a long-running server command stops when the client disconnects. Therefore:

- ordinary, scan, script and batch workloads have separate admission weights;
- blocking options such as `BLOCK` are parsed and capped below request timeout;
- unbounded blocking (`BLOCK 0`) is rejected;
- pipelines have command-count and aggregate argument-byte ceilings;
- scripts have stricter concurrency and size limits;
- replies have depth, element, scalar and exact encoded-byte limits;
- output truncation does not imply the Redis server produced less work;
- `KEYS`, large scans and other expensive reads remain available but receive
  explicit slow-command metrics and stricter concurrency, not semantic denial;
- production guidance strongly prefers replicas for exploratory high-cost reads.

## Caching

### Command catalog

The immutable parsed command catalog is cached per target and revision. It is
rebuilt every configured interval or after topology/server-version change.
Requests continue only while the catalog is within `command_catalog_max_age`.
A failed refresh retains no indefinite stale authorization; after max age the
target fails closed.

### Results

Redis result caching is disabled in the first release. Redis values and TTLs may
change rapidly, and generic command determinism is harder to prove. A future
cache may admit only commands whose catalog lacks nondeterministic tips and whose
key-version strategy is explicit.

## ACL and catalog re-attestation

Every five minutes by default, a maintenance-class task:

1. verifies server identity, version and deployment role;
2. obtains the current user's effective ACL;
3. verifies command and read-key scope did not broaden;
4. rebuilds or verifies the command catalog hash;
5. verifies module and Function catalog hashes;
6. atomically publishes a new capability snapshot on success.

Any broadening, unknown command, unsafe key spec or stale attestation marks the
target unhealthy. New requests fail closed. In-flight commands are allowed to
finish under their already acquired immutable capability snapshot and deadline.

## Audit and metrics

Audit events include:

- request ID and target alias;
- canonical command/subcommand;
- catalog revision;
- salted key-name fingerprints, never raw keys;
- key count and argument byte count;
- decision and stable reason code;
- workload class, duration and response bytes;
- element/scalar truncation;
- deployment role and redirect count.

They never include raw keys, argument values, script bodies, function arguments,
reply values, credentials, endpoints or Redis error text.

Metrics include admission wait, key-preflight latency, execution latency,
normalization latency, catalog age, attestation success, redirects, reply bytes,
truncations and stable rejection categories. Command names may be labels only
from the bounded attested catalog; module command labels are normalized to avoid
unbounded cardinality.

## Error handling

Errors expose stable categories such as:

- target not found or wrong engine;
- command unknown or not attested;
- command may modify data;
- command changes forbidden state;
- key extraction failed;
- key outside target scope;
- ACL/catalog attestation stale;
- cross-slot request rejected by Redis;
- concurrency or timeout exceeded;
- response budget exceeded;
- Redis rejected command.

Raw Redis error messages are logged only after secret-safe classification and
are not returned to MCP callers.

## Testing strategy

### Unit tests

- ACL rule parsing, selectors and category expansion;
- command metadata parsing across supported Redis versions;
- `RO/RW/OW/RM`, incomplete and variable key specifications;
- option-sensitive command variants;
- key extraction and pattern containment;
- RESP normalization, binary values, deep nesting and exact budgets;
- command and key fingerprints;
- blocking option caps;
- stale catalog and fail-closed health transitions.

### Hostile command corpus

At minimum:

- all string/hash/list/set/zset/stream mutation commands;
- `DEL`, `UNLINK`, expiry, rename, move and restore paths;
- `GETEX`, `SORT STORE`, `BITFIELD SET/INCRBY`;
- `XREADGROUP`, `XACK`, `XCLAIM`, `XAUTOCLAIM`;
- `EVAL`, `EVALSHA`, `FCALL` and scripts attempting nested writes;
- `FLUSHALL`, `FLUSHDB`, `SWAPDB`, `MIGRATE`, `COPY`;
- `CONFIG`, `ACL`, `CLIENT`, `MODULE`, `FUNCTION`, `SCRIPT` lifecycle;
- `PUBLISH`, subscription commands and `MONITOR`;
- `MULTI`, `WATCH`, `EXEC`, `DISCARD` injection;
- malformed arity, subcommands, key counts and binary arguments;
- module commands with missing or dishonest metadata;
- ACL patterns broader than target configuration.

Every rejected mutation is also executed through a direct test connection using
the provisioned MCP ACL user and must fail at the Redis boundary.

### Integration matrix

- supported Redis Open Source versions;
- RESP2 and RESP3;
- TLS and certificate failures;
- standalone primary and read-only replica;
- Sentinel failover;
- Cluster primary and replica routing, `MOVED`, `ASK` and cross-slot behavior;
- ACL changes during operation;
- command rename/disable behavior;
- allowed and unknown modules;
- read-only scripts attempting writes;
- cancellation, pool saturation and oversized nested replies.

### Fuzzing

- command vectors and subcommand normalization;
- RESP decoder normalization;
- command metadata and ACL response parsers;
- key specification extraction;
- pattern containment;
- script key-count framing;
- batch result accounting.

## Rollout plan

### Phase 1: Core model and standalone transport

- Split common, SQL and Redis capability interfaces.
- Add Redis configuration, credentials, TLS and driver.
- Add `redis_command` and `redis_batch` schemas.
- Implement RESP normalization and exact budgets.

### Phase 2: Security catalog

- Parse command metadata and effective ACLs.
- Implement key extraction, pattern containment and invariant denials.
- Add startup and periodic re-attestation.
- Complete hostile corpus and ACL boundary tests.

### Phase 3: Advanced reads

- Enable `SORT_RO`, `BITFIELD_RO`, scans and complex multi-key reads.
- Enable bounded `EVAL_RO`, `EVALSHA_RO` and attested `FCALL_RO`.
- Add signed module-command catalog support.

### Phase 4: Sentinel and Cluster

- Add discovery and topology admission.
- Add replica-read routing and internal `READONLY` setup.
- Test redirects, failover, catalog consistency and cross-slot behavior.

### Phase 5: Production gate

- Run the version/deployment integration matrix.
- Benchmark catalog preflight and pipeline overhead.
- Complete security review and operator provisioning guide.
- Mark Redis production-ready only after every acceptance criterion passes.

## Acceptance criteria

- A caller cannot execute a command that changes a key, TTL, stream consumer
  state, server state, topology, code catalog or externally visible channel.
- Direct mutation attempts fail both application policy and Redis ACL tests.
- Advanced core reads are admitted from live server metadata rather than a
  narrow handwritten list.
- `EVAL_RO`/`FCALL_RO` cannot invoke nested writes in integration tests.
- Every effective key is extracted and authorized before execution.
- ACL or command-catalog broadening makes the target fail closed within the
  configured recheck interval and never beyond maximum catalog age.
- Unknown Redis versions, modules, command flags or key specifications fail
  closed.
- SQL targets and behavior remain unchanged.
- Exact encoded response budgets hold for every RESP type and batch result.
- Tests, race detector, fuzz smoke tests and supported deployment integration
  tests pass.

## Risks and mitigations

| Risk | Mitigation |
| --- | --- |
| Redis upgrade changes `@read` | Expand and re-attest exact live catalog; pin tested versions |
| Keyless destructive command bypasses patterns | Invariant deny set and command capability intersection |
| Optional command form writes | Prefer `_RO` variants; reject variable write specs |
| Module lies about flags | Exact allowlisted module build and reviewed metadata snapshot |
| ACL drifts after startup | Periodic effective ACL attestation and maximum catalog age |
| Script attempts nested write | Only `_RO` variants plus Redis server enforcement |
| Script/KEYS blocks server | Separate admission, deadlines, slow metrics, replica guidance |
| Timeout causes automatic load amplification | No ambiguous automatic retry |
| Cluster redirect escapes configured topology | Admit redirects only from verified cluster topology |
| Binary/deep RESP exhausts memory | Streaming decode and depth/element/scalar/byte ceilings |
| Key names leak through logs | Salted fingerprints and counts only |
| `MULTI` leaks connection state | Managed dedicated connection with unconditional cleanup |

## Alternatives considered

### Static allowlist only

Rejected as the sole mechanism. It is simple but unnecessarily limits advanced
reads and becomes stale across Redis releases. A small invariant deny set still
exists, but eligibility primarily comes from the attested live catalog.

### Trust `+@read` only

Rejected. Categories evolve, keyless commands evade key patterns, and module
behavior differs. Category expansion is one input to attestation, not the proof.

### Replica-only security

Rejected. Redis documentation explicitly warns that a read-only replica still
has administrative commands unless ACLs remove them. Replica routing is useful
defense in depth, not authorization.

### Parse redis-cli command strings

Rejected. Redis is binary-safe and has no need for shell-like tokenization.
Structured command vectors remove quoting ambiguity and injection classes.

### Map Redis onto SQL tools

Rejected. Keys are not tables, Redis commands are not SELECT statements, and
fake schema/explain behavior would weaken both APIs.

## Open questions

1. Which exact Redis 8.x minors enter the first tested matrix?
2. Should production default to an explicit generated command list instead of
   `+@read`, even though startup expands and verifies the category?
3. Which Redis modules, if any, should receive first-party attestation profiles?
4. Should expensive keyspace-wide reads require a dedicated target pointed at a
   replica, or is weighted admission sufficient?
5. Which managed providers can safely expose self `ACL GETUSER`, and what signed
   attestation format should be used when they cannot?
6. Should atomic Redis batches be included in the first release or follow the
   standalone single-command security gate?

## References

- Redis ACL rules and read-key patterns:
  https://redis.io/docs/latest/operate/oss_and_stack/management/security/acl/
- `ACL SETUSER` rule semantics:
  https://redis.io/docs/latest/commands/acl-setuser/
- Redis command key specifications and `RO/RW/OW/RM` flags:
  https://redis.io/docs/latest/develop/reference/key-specs/
- `COMMAND INFO`:
  https://redis.io/docs/latest/commands/command-info/
- Redis read-only programmability:
  https://redis.io/docs/latest/develop/programmability/
- Redis replica read-only behavior and limitations:
  https://redis.io/docs/latest/operate/oss_and_stack/management/replication/
- Redis Cluster `READONLY`:
  https://redis.io/docs/latest/commands/readonly/

# RFC-0004: Redis Sentinel, Cluster, and Attested Module Support

- Status: Core implemented; module-specific index-prefix introspection remains
  fail-closed
- Authors: readonly-db-mcp maintainers
- Created: 2026-09-02
- Depends on: RFC-0003
- Scope: Redis Sentinel discovery, Redis Cluster routing, replica reads, and
  explicitly attested third-party module commands

## Summary

RFC-0003 deliberately fails closed for Sentinel, Cluster, and installed Redis
Modules. This RFC defines how to admit those modes without weakening the same
security objective: no public MCP tool may change Redis key data, key metadata,
TTLs, consumer state, server state, topology, code catalogs, or externally
observable channels.

The central rule is **attest before route**:

- a Sentinel-discovered instance is a candidate, not a trusted data node;
- a Cluster node returned by topology discovery, `MOVED`, or `ASK` is a
  candidate, not an automatically trusted redirect target;
- a module command that reports `readonly` is a candidate capability, not an
  automatically trusted command.

Candidates remain quarantined until their identity, role, TLS identity, ACL,
command catalog, key specifications, module inventory, version policy and target
scope all pass. Only then is an immutable routing/capability snapshot atomically
published to request handlers.

Sentinel and Cluster nodes must converge on a compatible safety catalog. Runtime
authorization uses the intersection of capabilities proven safe on every node
that may receive traffic, never their union. Failover or topology changes cannot
silently broaden permissions.

Third-party modules require exact, signed attestation profiles. Native modules
are part of the trusted computing base: Redis cannot independently prove that a
native module's implementation matches its declared `readonly` and key-spec
flags. Unknown, changed, unsigned, or incompletely described modules remain
rejected.

The generic implementation accepts `ordinary-keys`, `index-name`, and
`keyless-safe`. `index-prefix-attested` is rejected until a module-specific
introspection verifier exists. Exact artifact profiles also include the absolute
module path reported by `MODULE LIST`; the same regular file must be visible to
this process and its SHA-256 must match. This deliberately makes remote modules
without mounted artifact evidence unavailable instead of pretending that a
name/version pair proves native code identity.

Cluster replica targets attest both replicas and primaries because the driver
may send internal `COMMAND` key introspection to a primary. Primary clients have
a hook that permits only that application-inaccessible introspection command;
all data commands, pipelines and fallback reads are rejected. Every slot range
must expose at least one fully attested replica, so replica-only routing fails
closed instead of silently changing consistency.

The implementation publishes fully attested client generations under a
read/write lease: an in-flight request retains one matching client/policy pair,
and the retired pool closes only after those leases drain. A failed refresh
marks the target unhealthy, so stale topology is never used for new requests.

## Motivation

Production Redis deployments commonly need:

- Sentinel-based service discovery and automatic failover;
- Redis Cluster sharding and transparent slot migration;
- read scaling from replicas;
- RedisJSON, Redis Search, RedisTimeSeries or organization-specific modules.

Simply enabling go-redis Sentinel/Cluster routing is insufficient. A newly
promoted node may have a broader ACL, a different command catalog, an unknown
module build, an unexpected certificate, or a stale role. A Cluster redirect can
name an endpoint that was never configured. A module can label a native command
read-only while its implementation has side effects not represented in the
command metadata.

The existing standalone policy is retained. This RFC adds a verified topology
layer around it and a signed trust policy above module metadata.

## Goals

- Support `redis.mode: sentinel` with verified primary or replica discovery.
- Support `redis.mode: cluster` with verified slot routing, `MOVED`, `ASK`,
  resharding and optional replica reads.
- Never send caller traffic to a newly discovered endpoint before attestation.
- Require compatible ACL, key scope and command semantics across all routable
  nodes.
- Preserve advanced read-only Redis commands rather than limiting users to a
  small common-command list.
- Support explicitly reviewed module reads through signed module profiles.
- Detect topology, ACL, command, Function and module drift and fail closed.
- Keep endpoints, topology details, credentials and module paths out of MCP
  arguments and ordinary responses.
- Preserve admission, timeouts, response budgets, audit and metrics.
- Make failover behavior deterministic and observable.

## Non-goals

- Managing Sentinel quorum, initiating failover, or changing Sentinel config.
- Creating, repairing or resharding a Redis Cluster.
- Automatically synchronizing ACL files or users between nodes.
- Loading, upgrading or unloading modules or Functions.
- Trusting arbitrary Redis-compatible products as Redis Open Source nodes.
- Supporting write-capable module commands, even for temporary indexes or
  caches.
- Rewriting cross-slot commands into scatter/gather operations with changed
  semantics.
- Hiding staleness when reads are intentionally routed to replicas.
- Claiming that runtime metadata can prove a malicious native module harmless.
- Continuing on a partially verified topology merely to maximize availability.

## Security invariants

The following invariants apply to standalone, Sentinel and Cluster modes:

1. Every routable data node has passed the same node attestation contract.
2. Every request uses an immutable topology and capability snapshot.
3. Effective capabilities are the safe intersection across routable nodes.
4. Every effective key is read-only and inside configured key patterns.
5. Every effective ACL contains no write key patterns, channels or unsafe
   command grants.
6. Redirects and failovers cannot add a node without quarantine and attestation.
7. No topology/control command is exposed through `redis_command`.
8. Module commands require both live safe metadata and a trusted exact profile.
9. A failed or stale attestation removes the node or target from service.
10. No automatic retry may turn a topology transition into unbounded load.

Target availability is intentionally subordinate to data integrity and target
isolation.

## Architecture

```text
configured seeds / Sentinel quorum
                |
                v
        topology candidate set
                |
                v
      bounded quarantine workers
       /       |        |       \
     TLS     ROLE      ACL     command/module catalog
       \       |        |       /
                v
       immutable admitted nodes
                |
                v
      safe capability intersection
                |
                v
       atomic routing snapshot
                |
                v
   key preflight -> admission -> read command
```

The topology supervisor owns discovery and node lifecycles. Request goroutines
never mutate topology. They load one immutable snapshot, validate and route using
that snapshot, and release it when complete.

### New internal packages

```text
internal/dialects/redis/
  topology/
    supervisor.go
    snapshot.go
    node.go
    sentinel.go
    cluster.go
    redirect.go
  modules/
    profile.go
    verify.go
    embedded/
```

The existing standalone policy, ACL parser, result normalizer and command
execution path remain shared.

## Node attestation contract

Before a node becomes routable, a dedicated maintenance connection verifies:

- the endpoint belongs to configured seeds, an admitted Sentinel answer, or the
  verified Cluster topology;
- DNS resolution and IP/CIDR policy;
- TLS chain, hostname/SPIFFE identity and minimum version;
- Redis product and supported version;
- configured ACL username via `ACL WHOAMI`;
- effective ACL via self `ACL GETUSER` or signed provider attestation;
- exact read-key patterns and absence of channels/selectors/write grants;
- full command and subcommand catalog;
- command-key metadata and required internal introspection permissions;
- installed module inventory and profile matches;
- Function catalog revision when `FCALL_RO` is enabled;
- deployment role via `ROLE` and `INFO replication`;
- configured database selection where non-Cluster mode allows databases;
- expected cluster identity where applicable.

The result is a `NodeCapability` containing no credentials:

```go
type NodeCapability struct {
    NodeID             string
    EndpointID         string
    Role               NodeRole
    ServerVersion      string
    ACLRevision        string
    CommandRevision    string
    ModuleRevision     string
    FunctionRevision   string
    SafeCommands       ImmutableCommandSet
    ReadKeyPatterns    []string
    AttestedAt         time.Time
    ExpiresAt          time.Time
}
```

Network addresses are kept in internal routing state and are not returned by MCP
inspection tools.

### Compatibility intersection

For a set of routable nodes `N`, the target capability is:

```text
SafeTargetCommands = intersection(NodeCapability[n].SafeCommands for n in N)
```

ACL and key patterns must match exactly; they are not intersected silently. A
node with broader or narrower ACL scope indicates provisioning drift and remains
quarantined. Server versions may differ only within an explicitly tested rolling
upgrade window.

If a newly discovered node lacks one advanced read command, that command becomes
unavailable only after an explicitly valid topology snapshot is published. The
system never routes it optimistically and waits for Redis to reject it.

## Sentinel support

### Configuration

```yaml
targets:
  analytics-redis-sentinel:
    engine: redis
    environment: production
    consistency: current
    username: analytics_mcp_ro
    password_file: /run/secrets/analytics-redis.password
    redis:
      mode: sentinel
      database: 0
      key_patterns:
        - "analytics:*"
      sentinel:
        service_name: analytics-cache
        addresses:
          - sentinel-a.internal.example:26379
          - sentinel-b.internal.example:26379
          - sentinel-c.internal.example:26379
        username: readonly_discovery
        password_file: /run/secrets/analytics-sentinel.password
        min_agreement: 2
        discovery_timeout: 750ms
        refresh_interval: 5s
        endpoint_allowlist:
          dns_suffixes:
            - .internal.example
          cidrs:
            - 10.20.0.0/16
        read_role: primary
        require_master_link_up: true
        max_replica_lag_bytes: 16777216
      acl_recheck_interval: 1m
      command_catalog_max_age: 2m
    tls:
      mode: verify-full
      ca_file: /etc/readonly-db-mcp/redis-ca.pem
      server_name: redis.internal.example
```

Sentinel credentials are separate from data-node credentials. The discovery user
receives only the Sentinel read commands required for service discovery. It does
not receive Sentinel administration/failover commands.

### Discovery agreement

The official client algorithm accepts the first usable Sentinel answer and then
verifies the data-node role. This implementation adds a configurable agreement
gate for hostile or stale discovery protection:

1. Query configured Sentinels concurrently within a bounded timeout.
2. Authenticate and verify each Sentinel TLS identity.
3. Request the configured service's primary or replicas.
4. Canonicalize endpoint answers.
5. Require `min_agreement` Sentinels to report the same service epoch/endpoint
   set when enough Sentinels are reachable.
6. Reject endpoints outside configured DNS/CIDR policy.
7. Quarantine and attest each candidate data node.
8. Verify `ROLE` matches the requested read role.
9. Atomically publish only after the complete gate succeeds.

`min_agreement` must be at least two in production and cannot exceed the number
of configured Sentinel seeds. A single-Sentinel development escape hatch is
refused for production.

### Primary reads

For `read_role: primary`, the candidate must return primary/master from `ROLE`.
If Sentinel disconnects clients during reconfiguration, the next connection
restarts discovery and attestation. The old client generation is retired and
closed after in-flight reads finish or their deadlines expire.

### Replica reads

For `read_role: replica`:

- Sentinel replicas are filtered for healthy status;
- each candidate must return replica/slave from `ROLE`;
- `master_link_status` must be `up` when configured;
- the reported primary must belong to the agreed Sentinel service;
- replica priority and lag ceilings may be configured;
- responses report `consistency: eventual`;
- no fallback to a primary occurs unless explicitly configured and surfaced as
  a different target.

Primary and replica behavior should normally be separate target aliases. This
prevents a target's consistency semantics from changing during failure.

### Failover gate

When discovery changes:

```text
old admitted node -> draining
new endpoint       -> quarantined -> attested -> published
```

There is no interval in which caller traffic is sent to the new node before
attestation. If attestation fails, the target is unhealthy. It does not continue
using a node whose Sentinel role is no longer valid.

## Redis Cluster support

### Configuration

```yaml
targets:
  analytics-redis-cluster:
    engine: redis
    environment: production
    consistency: current
    username: analytics_mcp_ro
    password_file: /run/secrets/analytics-redis-cluster.password
    redis:
      mode: cluster
      database: 0
      key_patterns:
        - "analytics:{tenant-42}:*"
      cluster:
        seed_addresses:
          - redis-a.internal.example:6379
          - redis-b.internal.example:6379
          - redis-c.internal.example:6379
        topology_refresh_interval: 5s
        topology_max_age: 30s
        redirect_limit: 3
        read_role: primary
        endpoint_allowlist:
          dns_suffixes:
            - .internal.example
          cidrs:
            - 10.30.0.0/16
        require_full_slot_coverage: true
      acl_recheck_interval: 1m
      command_catalog_max_age: 2m
    tls:
      mode: verify-full
      ca_file: /etc/readonly-db-mcp/redis-ca.pem
      server_name: redis.internal.example
```

Cluster database must be zero. Caller `SELECT` remains forbidden.

### Bootstrap and topology admission

1. Connect only to configured seed addresses.
2. Verify TLS, Redis identity, ACL and command catalog on the seed.
3. Fetch `CLUSTER SHARDS` where supported, falling back to `CLUSTER SLOTS` only
   for an explicitly tested older version.
4. Validate slot ranges, node IDs, roles, endpoint formats and coverage.
5. Reject duplicate/conflicting slot ownership in one topology epoch.
6. Reject every endpoint outside configured DNS/CIDR policy.
7. Quarantine and attest every node eligible to receive traffic.
8. Require compatible ACL, commands, modules and Functions.
9. Publish an immutable slot table and capability intersection.

Topology commands are internal only and are never admitted through
`redis_command`.

### Routing

The existing exact command preflight resolves keys and their read-only flags.
Cluster mode additionally computes Redis hash slots from the exact binary keys.

- a one-key command routes to the owning admitted node;
- a multi-key command is sent unchanged when Redis Cluster permits its slot set;
- commands requiring one slot fail locally when keys span slots;
- the service does not split, merge or reorder a command;
- keyless commands follow an explicit request policy and never broadcast unless
  the command's attested semantics require it and the RFC profile permits it;
- atomic batches require every effective key to share one slot and one node;
- non-atomic pipelines are grouped by node while output order remains stable.

Hash-tag syntax remains an ordinary part of key names. Configuration validation
does not invent or rewrite hash tags.

### `MOVED` handling

`MOVED` indicates that a slot owner has changed. The redirect endpoint is not
used immediately:

1. stop retrying the caller request;
2. place the endpoint in quarantine;
3. trigger a bounded topology refresh from admitted nodes;
4. validate the new topology and candidate node;
5. publish a new snapshot;
6. retry the original read at most once when its deadline and retry budget allow.

The retry is safe from a data-integrity perspective because the command is
proven read-only, but it remains bounded to avoid load amplification.

### `ASK` handling

`ASK` is a temporary redirect during slot migration. Redis requires `ASKING` on
the target connection immediately before the command.

The implementation:

- accepts `ASK` only from an admitted source node;
- verifies the target endpoint belongs to the current or candidate topology;
- attests the target before use;
- uses a dedicated single-use connection;
- sends internal `ASKING` followed by exactly the already validated read command;
- never exposes `ASKING` to MCP callers;
- does not add the temporary target as a permanent owner without a topology
  refresh.

### Replica reads

Cluster replica mode requires:

- every replica to be mapped to an admitted primary/node ID;
- `ROLE` and replication-link verification;
- internal per-connection `READONLY` setup;
- no caller access to `READONLY` or `READWRITE`;
- compatible ACL/catalog/module revisions on replicas;
- explicit eventual consistency in target info and every result;
- bounded fallback behavior. Production defaults to no primary fallback.

### Partial topology

With `require_full_slot_coverage: true`, missing or unverified slots make the
entire target unhealthy. An optional future partial mode may reject only affected
slots, but it is not part of the first Cluster release because keyless and
multi-key command semantics become difficult to report safely.

## Endpoint security

Discovery is not authorization. Every discovered endpoint must satisfy:

- hostname/IP syntax and port ceilings;
- configured DNS suffix and CIDR allowlists;
- no loopback, link-local, multicast, unspecified or Unix socket endpoint unless
  the entire target is explicitly local development;
- DNS rebinding-resistant resolution: connect to the validated resolved address
  while preserving TLS server-name verification;
- certificate identity policy;
- no redirects to a different scheme, protocol or TLS mode;
- stable node ID binding after connection.

Endpoint validation occurs both when discovered and immediately before dialing.
Addresses are never accepted from MCP arguments.

## Topology snapshots and lifecycle

```go
type TopologySnapshot struct {
    Generation         uint64
    Mode               string
    ServiceOrClusterID string
    Nodes              map[NodeID]*AdmittedNode
    Slots              *SlotTable
    SafeCommands       ImmutableCommandSet
    CreatedAt          time.Time
    ExpiresAt          time.Time
}
```

Snapshots are immutable and reference-counted:

- request handlers load one atomic pointer;
- an update constructs a complete replacement off-path;
- successful publication increments the generation;
- old pools drain after their request references reach zero;
- forced close occurs after the maximum request timeout;
- snapshots older than `topology_max_age` fail closed.

There is no in-place mutation of slot maps or node capabilities.

## Module support

### Trust boundary

Redis Modules are native code. Runtime `COMMAND INFO`, ACL categories and key
specifications originate from the module itself. They are necessary inputs but
cannot prove that a malicious or buggy implementation behaves as declared.

Therefore enabling a module means the operator explicitly trusts an exact module
artifact and signed review profile as part of the server's trusted computing
base. Without that trust, the only safe behavior is rejection.

### Module profile

Profiles are versioned canonical JSON documents:

```json
{
  "profile_version": 1,
  "module": {
    "name": "search",
    "semantic_version": "x.y.z",
    "redis_compatibility": ["7.2", "7.4", "8.0"],
    "artifact_sha256": "...",
    "artifact_path": "/opt/redis/modules/redisearch.so",
    "vendor_build_id": "..."
  },
  "commands": {
    "FT.SEARCH": {
      "readonly": true,
      "key_model": "index-name",
      "external_side_effects": false,
      "blocking_class": "bounded-search",
      "maximum_reply_shape": "nested"
    },
    "FT.INFO": {
      "readonly": true,
      "key_model": "index-name",
      "external_side_effects": false
    }
  },
  "forbidden_commands": ["FT.CREATE", "FT.ALTER", "FT.DROPINDEX"],
  "catalog_sha256": "...",
  "tests_sha256": "...",
  "issued_at": "...",
  "expires_at": "...",
  "key_id": "redis-module-review-2026-01",
  "signature": "..."
}
```

The profile is signed with Ed25519. Trusted public keys are configured locally
or embedded for first-party profiles. Private signing keys never exist in the
runtime or repository.

### Artifact identity limitation

`MODULE LIST` exposes module identity/version information but does not provide a
cryptographic hash of the remote loaded binary. Consequently the runtime can
verify the signed profile, live module name/version/build identity and full
command catalog, but cannot independently hash a remote artifact.

Production admission therefore requires one of:

1. a deployment-platform attestation binding node image/module artifact digest
   to the Redis endpoint identity;
2. a managed-provider signed module build identity;
3. an operator acknowledgment that the exact module deployment is trusted,
   combined with live catalog matching and integration evidence.

The third option is explicit trust, not cryptographic proof. The target reports
which attestation level is active.

### Runtime module verification

For every node:

1. fetch `MODULE LIST` internally;
2. reject modules absent from configured signed profiles;
3. verify profile signature, validity period and Redis compatibility;
4. obtain module command names through `COMMAND LIST FILTERBY MODULE`;
5. load `COMMAND INFO` and `COMMAND DOCS` for every command/subcommand;
6. canonicalize and compare the live catalog with the signed digest;
7. ensure every admitted command reports read-only behavior;
8. require every effective key spec to be `RO`, complete and non-variable-write;
9. reject arbitrary-key, channel, blocking-without-yield and internal commands;
10. intersect admitted module commands across every routable node.

An unprofiled command added by a known module fails the entire module profile,
not merely that command. This prevents silent capability growth after upgrades.

### Key models beyond ordinary Redis keys

Some modules use logical objects such as search indexes whose names are not
ordinary Redis keys, or an index that indirectly reads a configured prefix.
Profiles must declare one of:

- `ordinary-keys`: live key extraction and `%R~` checks are sufficient;
- `index-name`: the logical object name has an explicit configured allowlist;
- `index-prefix-attested`: reserved for a future module-specific startup
  introspector; the generic implementation rejects it;
- `keyless-safe`: reviewed command reads no user data outside its reply metadata.

Unknown key models fail closed. A module command is never admitted merely because
`COMMAND GETKEYSANDFLAGS` returns no keys.

### Initial profiles

The first implementation may ship profiles for exact tested versions of:

- RedisJSON read commands such as JSON retrieval/type/length operations;
- Redis Search query/introspection commands whose indexes are prefix-attested;
- RedisTimeSeries range/query/info reads with ordinary-key validation.

The exact command list belongs in each signed profile and test corpus, not in
this RFC. Write/index-lifecycle commands remain forbidden.

### Module drift

A module inventory, version, catalog or profile change quarantines the node. In
Cluster/Sentinel mode it cannot rejoin routing until all compatibility checks
pass. Profile expiration fails closed.

## Functions and scripts across nodes

Read-only Functions introduce another distributed catalog:

- `FCALL_RO` is admitted only when the Function library/name, `no-writes` flag
  and code digest match across every routable node;
- failover candidates with missing or changed libraries remain quarantined;
- Cluster Functions must exist with identical digest on every possible target
  node;
- prefix-scoped script restrictions from RFC-0003 remain unless a separate
  script ACL prevents nested keyless scope escape;
- the service never loads or repairs Functions.

## Configuration model

New types:

```go
type RedisConfig struct {
    Mode          string
    Sentinel      RedisSentinelConfig
    Cluster       RedisClusterConfig
    ModuleProfiles []RedisModuleProfileRef
    // RFC-0003 fields remain.
}

type RedisModuleProfileRef struct {
    Path              string
    RequiredKeyID     string
    AttestationLevel  string
}
```

Strict decoding rejects Sentinel fields in Cluster mode and vice versa. Relative
profile/password paths resolve against the main config directory and receive the
same ownership/mode checks as credentials.

Hard ceilings cover seeds, discovered nodes, profiles, commands per profile,
topology refresh rate, redirects and concurrent quarantine work.

## Request execution

1. Resolve exact target and load one topology snapshot.
2. Reject stale/unhealthy snapshots.
3. Resolve command/subcommand in the snapshot capability intersection.
4. Decode and bound arguments.
5. Extract effective keys and access flags.
6. Validate configured key/module logical-object scope.
7. Compute slots and choose an admitted node.
8. Acquire admission before any Redis preflight or execution.
9. Execute exactly once, except one explicitly budgeted read-only topology retry.
10. Normalize and exactly bound the response.
11. Audit topology generation, capability revision and stable outcome.

The selected node is rechecked against the same snapshot immediately before
dialing. A request never switches to a node from a newer generation midway
through execution.

## Retry policy

Retries are allowed only for already proven read-only commands and only for:

- one `MOVED` after a successful new topology publication;
- one `ASK` against an attested temporary target;
- one Sentinel reconnect after a successfully attested generation change.

No retry occurs when:

- the caller deadline cannot cover the full gate;
- the command is a script/function whose server completion is ambiguous;
- the response has begun;
- catalog/module revisions changed incompatibly;
- the retry budget is exhausted.

## Caching

No Redis result cache is added.

Topology and capabilities have separate bounded caches:

- Sentinel answer cache: seconds, never beyond discovery max age;
- Cluster topology cache: seconds, invalidated on admitted `MOVED`;
- node capability cache: keyed by endpoint identity, node ID, server version,
  ACL revision, command revision and module revision;
- signed module profile cache: keyed by profile digest and signing key.

Cached attestation never authorizes a different certificate, node ID, module
revision or endpoint. Negative candidate results use a short cooldown to avoid
repeatedly hammering a bad node.

## Observability

Target inspection adds non-secret fields:

- deployment mode and read role;
- topology generation and age;
- admitted/quarantined/draining node counts;
- slot coverage percentage for Cluster;
- capability intersection revision;
- module profile names, versions and attestation levels;
- last successful discovery and full attestation time.

Audit events include target, command, salted key fingerprints, topology
generation, abstract node ID, route role, redirect type/count, capability/module
revision and decision. They exclude endpoints, raw node IDs, Sentinel answers,
keys, arguments, replies, module paths and credentials.

Metrics include discovery agreement, candidate attestation latency/outcome,
topology age, generation changes, admitted node counts, slot coverage, redirects,
failover downtime, profile expiry and capability-intersection size.

## Failure behavior

| Condition | Behavior |
| --- | --- |
| Sentinels disagree below quorum | Target unhealthy; retain no stale routing beyond max age |
| Candidate role is wrong | Quarantine candidate and rediscover |
| Candidate ACL is broader | Quarantine; never intersect ACL scope silently |
| Candidate lacks a safe command | Publish only a valid safe intersection or fail configured minimum capability gate |
| Cluster redirect endpoint outside policy | Reject request and quarantine topology update |
| Slot coverage incomplete | Target unhealthy in first release |
| Module unknown or profile invalid | Quarantine node/target |
| Module catalog differs from profile | Quarantine node/target |
| Function digest differs | Remove `FCALL_RO` from intersection or fail required capability gate |
| Topology/capability snapshot expires | Reject new requests |
| Old generation still has requests | Drain until request deadline, then close |

## Testing strategy

### Unit tests

- Sentinel reply parsing, agreement and service-name binding;
- endpoint DNS/CIDR/TLS policy and rebinding defense;
- `ROLE` and replication-state classification;
- Cluster shard/slot parsing, overlap/gap detection and slot hashing;
- immutable snapshot publication and draining;
- `MOVED`/`ASK` parsing, quarantine and retry budgets;
- capability intersection across version/catalog differences;
- module profile canonicalization, Ed25519 signatures and expiration;
- module live-catalog comparison and logical key models;
- cross-slot atomic batch rejection;
- stale snapshot fail-closed behavior.

### Sentinel integration

- three Sentinels, one primary and at least two replicas;
- primary failover during idle, preflight and active read;
- stale Sentinel answer and quorum disagreement;
- candidate with wrong role, ACL, certificate, module or Function digest;
- replica-only target with link-down and lag failures;
- discovery endpoint outside allowlist;
- old pool drain without routing new work to it.

### Cluster integration

- at least three primaries and replicas;
- complete routing across all 16,384 slots;
- online reshard producing `MOVED` and `ASK`;
- malicious/out-of-policy redirect endpoint;
- node replacement with broader ACL;
- rolling compatible and incompatible Redis upgrades;
- replica reads with internal `READONLY`;
- same-slot and cross-slot multi-key commands and batches;
- topology gaps, overlaps and stale epochs.

### Module integration

- exact approved module versions and signed profiles;
- unknown module, version drift and extra command drift;
- forged, expired and wrong-key profile signatures;
- module command reporting write or incomplete key specs;
- hostile write commands rejected by application and ACL;
- before/after keyspace digest proving admitted commands do not change logical
  data in the tested module/version matrix;
- native module crash and timeout containment.

### Fuzzing and fault injection

- Sentinel and Cluster nested RESP topology replies;
- redirect strings, IPv6 endpoints and malformed node IDs;
- overlapping and extreme slot ranges;
- module profile JSON and signature envelopes;
- command metadata/key specs from modules;
- concurrent topology publication, cancellation and shutdown;
- network partitions, delayed DNS, TLS rotation and partial pool failure.

## Rollout plan

### Phase A: Shared verified topology layer

- Implement endpoint policy, node attestation and immutable snapshots.
- Refactor standalone mode to use a one-node snapshot.
- Add generation-aware execution and pool draining.

### Phase B: Sentinel primary

- Add multi-Sentinel agreement and role verification.
- Add quarantine-before-publish failover.
- Complete primary failover integration tests.

### Phase C: Sentinel replicas

- Add replica selection, link/lag gates and eventual consistency reporting.
- Keep primary/replica targets distinct by default.

### Phase D: Cluster primaries

- Add topology/slot validation and primary routing.
- Add bounded verified `MOVED` and `ASK` handling.
- Complete reshard and hostile redirect tests.

### Phase E: Cluster replicas

- Add replica mappings, internal `READONLY`, staleness gates and no-fallback
  defaults.

### Phase F: Module profiles

- Implement canonical signed profile format and trusted-key store.
- Ship only exact profiles that pass independent review and integration tests.
- Keep all other modules fail closed.

## Acceptance criteria

- No caller command reaches a discovered node before full attestation.
- Sentinel failover cannot introduce a node with broader ACL, unknown commands,
  modules, Functions or TLS identity.
- Cluster `MOVED`/`ASK` cannot redirect outside verified topology/endpoint policy.
- Every routable node satisfies exact ACL/key scope and the published command
  capability is their safe intersection.
- Cluster routing preserves Redis command semantics and never silently performs
  scatter/gather rewriting.
- Replica reads never silently fall back to a primary when the target promises
  eventual replica reads.
- Unknown, changed or unsigned modules fail closed.
- Every admitted module command matches an unexpired signed profile and live
  read-only key metadata on every routable node.
- Prefix/logical-object boundaries hold for module queries.
- Direct and module mutation attempts fail both policy and Redis ACL/integration
  boundary tests.
- Topology, ACL and module drift becomes unavailable within configured max age.
- Existing standalone, MySQL and PostgreSQL behavior remains unchanged.
- Full tests, race detector, fuzz smoke tests and failover/reshard integration
  matrix pass.

## Risks and mitigations

| Risk | Mitigation |
| --- | --- |
| Sentinel returns stale/hostile endpoint | Multi-Sentinel agreement, endpoint policy, `ROLE`, quarantine |
| Failover briefly routes before verification | Build new client generation off-path; publish only after attestation |
| ACLs differ across Cluster nodes | Verify every routable node; exact ACL equality |
| `MOVED`/`ASK` enables SSRF | Endpoint allowlist and attestation before redirect use |
| Slot map is incomplete/conflicting | Full coverage and overlap validation; fail closed |
| Replica becomes primary | Role recheck, connection retirement, distinct target semantics |
| Rolling upgrade changes commands | Safe capability intersection and tested version window |
| Module lies about readonly metadata | Exact trusted artifact/profile is part of TCB; unknown modules rejected |
| Remote module binary cannot be hashed | Deployment/provider attestation or explicit operator trust level |
| Module logical index spans forbidden keys | Profile key model plus startup index-prefix attestation |
| Topology churn causes load storm | Singleflight refresh, bounded quarantine workers, cooldowns |
| Retry amplifies expensive reads | One topology retry, deadlines and no script retry |
| Old pool races with new generation | Immutable reference-counted generations and bounded drain |

## Alternatives considered

### Use go-redis automatic routing without a supervisor

Rejected. It can discover or redirect to a node before application-level ACL,
catalog, module and endpoint attestation completes.

### Trust Sentinel as the complete authorization source

Rejected. Sentinel is a discovery/configuration authority. The official client
flow still verifies the data-node role, and this project additionally requires
TLS, ACL and capability proof.

### Trust identical configuration management across nodes

Rejected as the only control. Configuration management is valuable, but runtime
attestation detects missed nodes, partial rollouts and drift.

### Trust module `readonly` flags

Rejected. Native module code supplies those flags itself. Live metadata is
required but not sufficient without an exact trusted profile/artifact policy.

### Permit unknown modules but block their commands

Rejected for the production default. An unknown native module expands the
server's trusted code and may affect core behavior. A future explicit
`installed_but_unreachable` policy would require separate review.

### Prefer availability and route through partially verified nodes

Rejected. This server's primary guarantee is non-mutation and scoped access;
partial verification cannot preserve that guarantee.

## Open questions

1. Should Sentinel production agreement require a majority of configured seeds
   or a configurable minimum of two?
2. Which node workload identity mechanism should complement TLS hostnames in
   Kubernetes deployments?
3. Should a compatible rolling upgrade reduce the command intersection or make
   the target unavailable until all nodes converge?
4. Which provider/build attestation formats can cryptographically bind a remote
   module artifact to a Redis endpoint?
5. Which exact RedisJSON, Search and TimeSeries versions should receive initial
   signed profiles?
6. Should profile signing keys be only operator-configured, or may the project
   embed a separately governed first-party review key?
7. Is a future partial-slot Cluster mode worth its added keyless/multi-key
   semantics and operational complexity?

## References

- Redis Sentinel client specification:
  https://redis.io/docs/latest/develop/reference/sentinel-clients/
- Redis Sentinel security and operation:
  https://redis.io/docs/latest/operate/oss_and_stack/management/sentinel/
- Redis Cluster specification and `ASK`/`MOVED` semantics:
  https://redis.io/docs/latest/operate/oss_and_stack/reference/cluster-spec/
- Redis Cluster `READONLY`:
  https://redis.io/docs/latest/commands/readonly/
- Redis command key specifications:
  https://redis.io/docs/latest/develop/reference/key-specs/
- Redis Modules API command flags and key specifications:
  https://redis.io/docs/latest/develop/reference/modules/modules-api-ref/
- Redis ACL behavior and key permissions:
  https://redis.io/docs/latest/operate/oss_and_stack/management/security/acl/

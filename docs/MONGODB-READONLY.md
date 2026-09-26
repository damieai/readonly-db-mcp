# MongoDB read-only connector, P2

`mongodb_read` accepts an exact configured target and collection. It exposes
`metadata` (collection index descriptions), native `find`, exact `count`, and
a reviewed subset of native aggregation. Input and result documents use
MongoDB Extended JSON. Results use **canonical** Extended JSON, so ObjectIDs,
dates and 64-bit integers survive JSON transport without precision loss.

Copy [the example configuration](../configs/mongodb.example.yaml). The target
pins an exact MongoDB 8.0 patch version (8.0.32 or newer), one direct MongoDB
host and port, one database, and exact collection names. Production requires
TLS verification against a configured CA. Plaintext is accepted only on
loopback for a non-production target. Replica-set discovery and automatic
driver retries are unavailable in this first profile; the driver cannot
connect to an unreviewed advertised host.

Create a custom role on the target database granting **only** `find` and
`listIndexes` on each configured collection. Use a dedicated SCRAM-SHA-256
user, usually created in `admin`. A built-in database-wide `read` role does
not satisfy the exact collection-scope proof. At startup and periodically,
the connector checks `buildInfo`, inspects the authenticated identity's
effective privileges with `connectionStatus(showPrivileges: true)`, and
compares the authorized collection catalog with the configured list. Views,
other databases, database-wide grants, write actions and extra collections
fail closed. The catalog request uses `authorizedCollections: true` and
`nameOnly: true`, which [MongoDB permits with collection-scoped read access](https://www.mongodb.com/docs/manual/reference/command/listcollections/).

Native `find` supports filter, projection, sort, hint, let, min/max,
collation, skip and limit. For example:

```json
{
  "target": "agent-documents",
  "collection": "docs",
  "operation": "find",
  "body": {
    "filter": {"tenant": "team-a", "_id": {"$oid": "507f1f77bcf86cd799439011"}},
    "projection": {"title": 1, "source_id": 1},
    "sort": {"created_at": -1},
    "limit": 10
  }
}
```

Aggregation accepts read-only stages including match, projection, computed
fields, grouping, sorting, window functions, bucketing, faceting, unwind,
densify/fill, geospatial read, sampling, and `$lookup`, `$graphLookup` and
`$unionWith` against exact configured collections. Nested pipelines are
checked recursively. `$out`, `$merge`, server-side JavaScript (`$where`,
`$function`, `$accumulator` and `$code`), `$currentOp`, `$search` and
`$vectorSearch` are unavailable. Search and vector search require separate
versioned profiles and live index proof. MongoDB documents `$out`/`$merge`
as writing stages and `$lookup`/`$unionWith` as cross-collection reads in its
[aggregation reference](https://www.mongodb.com/docs/manual/reference/operator/aggregation-pipeline/).

Every query passes strict JSON parsing, a request/stage ceiling, admission,
a whole-call deadline and output-byte cap. `find` and aggregate append or set
a server-side limit of at most `max_rows + 1`; the extra item marks a truncated
result. Aggregation disables disk spill. A cursor is closed after use, including
on cancellation. Count can still scan a collection, so the deadline bounds
its server work.

## Reproducible live acceptance

On a Docker host, create a disposable authenticated fixture. Keep the admin
password outside this MCP process:

```sh
export MONGODB_ADMIN_PASSWORD="$(openssl rand -hex 32)"
export MONGODB_LIVE_PASSWORD="$(openssl rand -hex 32)"
docker run -d --rm --name readonly-mongodb-p2 \
  -p 127.0.0.1:27017:27017 \
  -e MONGO_INITDB_ROOT_USERNAME=root \
  -e MONGO_INITDB_ROOT_PASSWORD="$MONGODB_ADMIN_PASSWORD" \
  mongo:8.0.32
docker image inspect mongo:8.0.32 --format '{{index .RepoDigests 0}}'
```

Once startup completes, create a collection and a separate private collection,
then a role whose effective grants name only `docs`:

```sh
docker exec -e MONGODB_LIVE_PASSWORD readonly-mongodb-p2 \
  mongosh --quiet -u root -p "$MONGODB_ADMIN_PASSWORD" \
  --authenticationDatabase admin --eval '
    const data = db.getSiblingDB("agent_data");
    data.docs.insertMany([
      {tenant:"team-a", source_id:"doc-1", title:"first"},
      {tenant:"team-b", source_id:"doc-2", title:"second"}
    ]);
    data.private.insertOne({secret:"fixture-only"});
    data.createRole({role:"agentExactRead", privileges:[
      {resource:{db:"agent_data",collection:"docs"},
       actions:["find","listIndexes"]}
    ], roles:[]});
    db.getSiblingDB("admin").createUser({user:"agent_ro",
      pwd:process.env.MONGODB_LIVE_PASSWORD,
      roles:[{role:"agentExactRead",db:"agent_data"}]});
  '
```

Run the opt-in live test. It exercises metadata, find, count, aggregation,
cancelled reads, a denied native update of an absent ID, and denial of an
out-of-scope collection read:

```sh
export MONGODB_LIVE_HOST=127.0.0.1
export MONGODB_LIVE_PORT=27017
export MONGODB_LIVE_USER=agent_ro
export MONGODB_LIVE_VERSION=8.0.32
go test ./internal/dialects/mongodb -run '^TestMongoDBLiveReadOnlyGate$' -v
```

Record the image digest, server version, role descriptor, test output, latency
and resource observations before declaring this build live-qualified. Local
policy and cursor tests do not establish native mutation denial on a real
server.

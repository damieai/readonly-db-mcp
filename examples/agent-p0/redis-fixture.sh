#!/bin/sh
set -eu

# Run with a disposable administrator identity. REDISCLI_AUTH carries its secret.
# REDIS_HOST and REDIS_PORT default to a local fixture service.
: "${REDIS_HOST:=127.0.0.1}"
: "${REDIS_PORT:=6379}"

redis_cli() {
  redis-cli -h "$REDIS_HOST" -p "$REDIS_PORT" "$@"
}

redis_cli FT.CREATE search-agent-incidents ON HASH PREFIX 1 tenant:agent:incident: SCHEMA service TAG embedding VECTOR FLAT 6 TYPE FLOAT32 DIM 3 DISTANCE_METRIC L2

for incident in 101 102 103; do
  case "$incident" in
    101) service=checkout; vector='1,0,0' ;;
    102) service=checkout; vector='0.8,0.6,0' ;;
    103) service=catalog; vector='0,1,0' ;;
  esac
  redis_cli HSET "tenant:agent:incident:$incident" service "$service"
  python3 -c 'import struct,sys; sys.stdout.buffer.write(struct.pack("<3f", *(float(x) for x in sys.argv[1].split(","))))' "$vector" |
    redis_cli -x HSET "tenant:agent:incident:$incident" embedding
done

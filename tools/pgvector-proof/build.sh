#!/bin/sh
set -eu
: "${PG_CONFIG:?Set PG_CONFIG to the PostgreSQL 16 pg_config path}"
output=${1:-bin/pgvector-proof.so}
source=${2:-tools/pgvector-proof/proof.c}
case "$("$PG_CONFIG" --version)" in
  "PostgreSQL 16."*) ;;
  *) echo 'PostgreSQL 16 headers are required' >&2; exit 1 ;;
esac
mkdir -p "$(dirname "$output")"
cc -std=c99 -D_GNU_SOURCE -O2 -fPIC -shared -Wall -Wextra -Werror \
  -Wno-unused-parameter -I"$("$PG_CONFIG" --includedir-server)" \
  -I"$("$PG_CONFIG" --includedir)" "$source" -o "$output"

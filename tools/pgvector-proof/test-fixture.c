/* Disposable native test fixture only. Never install in a runtime database. */
#include "postgres.h"
#include "fmgr.h"

PG_MODULE_MAGIC;
PG_FUNCTION_INFO_V1(mcp_test_tripwire);
PG_FUNCTION_INFO_V1(mcp_test_tripwire_count);

static int64 calls = 0;

Datum mcp_test_tripwire(PG_FUNCTION_ARGS) {
    calls++;
    ereport(ERROR, (errmsg("test callback executed")));
    PG_RETURN_NULL();
}

Datum mcp_test_tripwire_count(PG_FUNCTION_ARGS) {
    PG_RETURN_INT64(calls);
}

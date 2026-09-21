/* RFC-0007 P1 native analysis prototype, PostgreSQL 16 only.
 * No SPI execution or planner invocation. Not a standalone security boundary:
 * native analysis can invoke type input/typmod callbacks. A production caller
 * must prove those dependencies before analysis (remaining P1 admission gate).
 */
#include "postgres.h"
#include "fmgr.h"
#include "varatt.h"
#include "miscadmin.h"
#include "catalog/pg_class_d.h"
#include "catalog/pg_operator_d.h"
#include "catalog/pg_proc_d.h"
#include "catalog/pg_type_d.h"
#include "common/cryptohash.h"
#include "common/sha2.h"
#include "lib/stringinfo.h"
#include "nodes/nodeFuncs.h"
#include "nodes/parsenodes.h"
#include "parser/parser.h"
#include "parser/analyze.h"
#include "rewrite/rewriteHandler.h"
#include "utils/builtins.h"
#include "utils/lsyscache.h"

#if PG_VERSION_NUM < 160000 || PG_VERSION_NUM >= 170000
#error "The analysis helper is reviewed for PostgreSQL 16 only"
#endif

PG_MODULE_MAGIC;
PG_FUNCTION_INFO_V1(mcp_vector_bind);

typedef struct Ref { Oid classid; Oid oid; } Ref;
typedef struct Proof {
    Ref refs[8192];
    int count;
    int nodes;
    int depth;
} Proof;

static void add_ref(Proof *p, Oid classid, Oid oid) {
    int i;
    if (!OidIsValid(oid)) return;
    for (i=0;i<p->count;i++)
        if (p->refs[i].classid==classid && p->refs[i].oid==oid) return;
    if (p->count>=8192) ereport(ERROR,(errmsg("binding reference limit")));
    p->refs[p->count].classid=classid;
    p->refs[p->count++].oid=oid;
}

static void add_operator(Proof *p, Oid oid) {
    if (!OidIsValid(oid)) return;
    add_ref(p,OperatorRelationId,oid);
    add_ref(p,ProcedureRelationId,get_opcode(oid));
}

static void enter(Proof *p) {
    CHECK_FOR_INTERRUPTS();
    check_stack_depth();
    if (++p->nodes>100000 || ++p->depth>256)
        ereport(ERROR,(errmsg("binding tree resource limit")));
}

static bool raw_check(Node *node, Proof *p) {
    bool result;
    if (!node) return false;
    enter(p);
    if (IsA(node,ParamRef) && (((ParamRef *)node)->number<1 || ((ParamRef *)node)->number>10000))
        ereport(ERROR,(errmsg("binding parameter limit")));
    if (IsA(node,SelectStmt)) {
        SelectStmt *s=(SelectStmt *)node;
        if (s->intoClause || s->lockingClause)
            ereport(ERROR,(errmsg("binding requires a non-locking read")));
    }
    if (IsA(node,InsertStmt) || IsA(node,UpdateStmt) || IsA(node,DeleteStmt) || IsA(node,MergeStmt))
        ereport(ERROR,(errmsg("binding rejects modifying statements")));
    result=raw_expression_tree_walker(node,raw_check,p);
    p->depth--;
    return result;
}

static bool bound_walk(Node *node, Proof *p) {
    bool result;
    if (!node) return false;
    enter(p);
    if (IsA(node,Query)) {
        Query *q=(Query *)node;
        ListCell *cell;
        if (q->commandType!=CMD_SELECT || q->hasModifyingCTE || q->rowMarks || q->utilityStmt)
            ereport(ERROR,(errmsg("rewritten query is not a read")));
        foreach(cell,q->rtable) {
            RangeTblEntry *rte=lfirst_node(RangeTblEntry,cell);
            if (rte->rtekind==RTE_RELATION) add_ref(p,RelationRelationId,rte->relid);
        }
        result=query_tree_walker(q,bound_walk,p,QTW_EXAMINE_SORTGROUP);
        p->depth--;
        return result;
    }
    switch (nodeTag(node)) {
    case T_FuncExpr:
        add_ref(p,ProcedureRelationId,((FuncExpr *)node)->funcid);break;
    case T_OpExpr: case T_DistinctExpr: case T_NullIfExpr:
        add_operator(p,((OpExpr *)node)->opno);break;
    case T_ScalarArrayOpExpr:
        add_operator(p,((ScalarArrayOpExpr *)node)->opno);break;
    case T_Aggref:
        add_ref(p,ProcedureRelationId,((Aggref *)node)->aggfnoid);break;
    case T_WindowFunc:
        add_ref(p,ProcedureRelationId,((WindowFunc *)node)->winfnoid);break;
    case T_RowCompareExpr: {
        ListCell *cell;
        foreach(cell,((RowCompareExpr *)node)->opnos) add_operator(p,lfirst_oid(cell));
        break;
    }
    case T_SortGroupClause:
        add_operator(p,((SortGroupClause *)node)->eqop);
        add_operator(p,((SortGroupClause *)node)->sortop);break;
    case T_CoerceViaIO: {
        CoerceViaIO *c=(CoerceViaIO *)node;
        Oid input,param,output;
        bool variable;
        getTypeInputInfo(c->resulttype,&input,&param);
        getTypeOutputInfo(exprType((Node *)c->arg),&output,&variable);
        add_ref(p,ProcedureRelationId,input);
        add_ref(p,ProcedureRelationId,output);break;
    }
    case T_NextValueExpr:
        ereport(ERROR,(errmsg("binding rejects sequence mutation")));break;
    default: break;
    }
    switch(nodeTag(node)) {
    case T_Var: case T_Const: case T_Param: case T_FuncExpr:
    case T_OpExpr: case T_DistinctExpr: case T_NullIfExpr:
    case T_Aggref: case T_WindowFunc: case T_ArrayExpr: case T_RowExpr:
    case T_RelabelType: case T_CoerceViaIO: case T_ArrayCoerceExpr:
    case T_CoerceToDomain: case T_CaseExpr: case T_CoalesceExpr:
    case T_MinMaxExpr: case T_SubscriptingRef: case T_FieldSelect:
        add_ref(p,TypeRelationId,exprType(node));break;
    default: break;
    }
    result=expression_tree_walker(node,bound_walk,p);
    p->depth--;
    return result;
}

Datum mcp_vector_bind(PG_FUNCTION_ARGS) {
    text *input=PG_GETARG_TEXT_PP(0);
    char *sql;
    List *raw,*rewritten;
    ListCell *cell;
    RawStmt *stmt;
    Query *query;
    Oid *parameters=NULL;
    int count=0,i;
    Proof *proof;
    StringInfoData output;
    pg_cryptohash_ctx *hash;
    uint8 digest[PG_SHA256_DIGEST_LENGTH];
    char hex[PG_SHA256_DIGEST_LENGTH*2+1];
    if (VARSIZE_ANY_EXHDR(input)>1024*1024)
        ereport(ERROR,(errmsg("binding SQL byte limit")));
    sql=text_to_cstring(input);
    hash=pg_cryptohash_create(PG_SHA256);
    if(!hash || pg_cryptohash_init(hash)<0 ||
       pg_cryptohash_update(hash,(const uint8 *)sql,strlen(sql))<0 ||
       pg_cryptohash_final(hash,digest,sizeof(digest))<0)
        ereport(ERROR,(errmsg("binding digest failure")));
    pg_cryptohash_free(hash);
    for(i=0;i<PG_SHA256_DIGEST_LENGTH;i++) snprintf(hex+i*2,3,"%02x",digest[i]);
    proof=palloc0(sizeof(Proof));
    raw=raw_parser(sql,RAW_PARSE_DEFAULT);
    if (list_length(raw)!=1) ereport(ERROR,(errmsg("binding requires one statement")));
    stmt=linitial_node(RawStmt,raw);
    if (!IsA(stmt->stmt,SelectStmt)) ereport(ERROR,(errmsg("binding requires SELECT")));
    raw_check(stmt->stmt,proof);
    query=parse_analyze_varparams(stmt,sql,&parameters,&count,NULL);
    if(count>10000) ereport(ERROR,(errmsg("binding parameter limit")));
    rewritten=QueryRewrite(query);
    foreach(cell,rewritten) bound_walk((Node *)lfirst(cell),proof);
    initStringInfo(&output);
    appendStringInfo(&output,"{\"format\":1,\"postgresql_major\":16,\"sql_sha256\":\"%s\",\"database_oid\":%u,\"role_oid\":%u,\"parameters\":[",hex,MyDatabaseId,GetUserId());
    for(i=0;i<count;i++) {
        if(!OidIsValid(parameters[i]) || parameters[i]==UNKNOWNOID)
            ereport(ERROR,(errmsg("binding parameter type is unresolved")));
        appendStringInfo(&output,"%s%u",i?",":"",parameters[i]);
        add_ref(proof,TypeRelationId,parameters[i]);
    }
    appendStringInfoString(&output,"],\"objects\":[");
    for(i=0;i<proof->count;i++) {
        const char *class_name;
        switch(proof->refs[i].classid) {
        case ProcedureRelationId: class_name="pg_proc";break;
        case OperatorRelationId: class_name="pg_operator";break;
        case TypeRelationId: class_name="pg_type";break;
        case RelationRelationId: class_name="pg_class";break;
        default: elog(ERROR,"unexpected proof class");class_name="";
        }
        appendStringInfo(&output,"%s{\"class\":\"%s\",\"oid\":%u}",i?",":"",class_name,proof->refs[i].oid);
    }
    appendStringInfoString(&output,"]}");
    PG_RETURN_TEXT_P(cstring_to_text_with_len(output.data,output.len));
}

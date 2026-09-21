-- Canonical object descriptors for the reviewed pgvector extension. $1 is its
-- exact installation schema. Run inside a read-only transaction with search_path
-- pg_catalog so regtype/regprocedure renderings cannot be shadowed.
WITH e AS (
  SELECT x.oid, x.extnamespace FROM pg_catalog.pg_extension x
  JOIN pg_catalog.pg_namespace n ON n.oid=x.extnamespace
  WHERE x.extname='vector' AND n.nspname=$1
), members AS (
  SELECT d.classid,d.objid,d.objsubid FROM pg_catalog.pg_depend d,e
  WHERE d.refclassid='pg_catalog.pg_extension'::regclass
    AND d.refobjid=e.oid AND d.deptype='e'
), objects AS (
  SELECT classid,objid FROM members
  UNION SELECT 'pg_catalog.pg_type'::regclass,t.typarray FROM pg_catalog.pg_type t
    JOIN members m ON m.classid='pg_catalog.pg_type'::regclass AND m.objid=t.oid WHERE t.typarray<>0
  UNION SELECT 'pg_catalog.pg_amop'::regclass,a.oid FROM pg_catalog.pg_amop a
    JOIN members m ON m.classid='pg_catalog.pg_opfamily'::regclass AND m.objid=a.amopfamily
  UNION SELECT 'pg_catalog.pg_amproc'::regclass,a.oid FROM pg_catalog.pg_amproc a
    JOIN members m ON m.classid='pg_catalog.pg_opfamily'::regclass AND m.objid=a.amprocfamily
), descriptors AS (
 SELECT o.classid,o.objid,
 CASE o.classid
 WHEN 'pg_catalog.pg_proc'::regclass THEN (
   SELECT jsonb_build_object('signature',p.oid::regprocedure::text,
     'definition',CASE WHEN p.prokind<>'a' THEN pg_get_functiondef(p.oid) ELSE NULL END,
     'support',p.prosupport::regprocedure::text,'kind',p.prokind,
     'volatility',p.provolatile,'security_definer',p.prosecdef,
     'leakproof',p.proleakproof,'strict',p.proisstrict,'parallel',p.proparallel,
     'returns_set',p.proretset,'cost',p.procost,'rows',p.prorows,'config',p.proconfig,
     'aggregate',CASE WHEN a.aggfnoid IS NULL THEN NULL ELSE
       (to_jsonb(a)-ARRAY['aggfnoid','aggtranstype','aggmtranstype','aggsortop']) ||
       jsonb_build_object('aggtranstype',a.aggtranstype::regtype::text,
         'aggmtranstype',a.aggmtranstype::regtype::text,'aggsortop',a.aggsortop::regoperator::text,
         'aggtransfn',a.aggtransfn::regprocedure::text,'aggfinalfn',a.aggfinalfn::regprocedure::text,
         'aggcombinefn',a.aggcombinefn::regprocedure::text,'aggserialfn',a.aggserialfn::regprocedure::text,
         'aggdeserialfn',a.aggdeserialfn::regprocedure::text,'aggmtransfn',a.aggmtransfn::regprocedure::text,
         'aggminvtransfn',a.aggminvtransfn::regprocedure::text,'aggmfinalfn',a.aggmfinalfn::regprocedure::text)
       END)
   FROM pg_catalog.pg_proc p LEFT JOIN pg_catalog.pg_aggregate a ON a.aggfnoid=p.oid WHERE p.oid=o.objid)
 WHEN 'pg_catalog.pg_type'::regclass THEN (
   SELECT (to_jsonb(t)-ARRAY['oid','typnamespace','typowner','typacl','typrelid','typelem','typarray','typbasetype','typcollation']) ||
     jsonb_build_object('typelem',t.typelem::regtype::text,'typarray',t.typarray::regtype::text,
       'typbasetype',t.typbasetype::regtype::text,'typrelid',t.typrelid::regclass::text,
       'typcollation',t.typcollation::regcollation::text,
       'typinput',t.typinput::regprocedure::text,'typoutput',t.typoutput::regprocedure::text,
       'typreceive',t.typreceive::regprocedure::text,'typsend',t.typsend::regprocedure::text,
       'typmodin',t.typmodin::regprocedure::text,'typmodout',t.typmodout::regprocedure::text,
       'typanalyze',t.typanalyze::regprocedure::text,'typsubscript',t.typsubscript::regprocedure::text)
   FROM pg_catalog.pg_type t WHERE t.oid=o.objid)
 WHEN 'pg_catalog.pg_operator'::regclass THEN (
   SELECT (to_jsonb(p)-ARRAY['oid','oprnamespace','oprowner','oprleft','oprright','oprresult','oprcom','oprnegate']) ||
     jsonb_build_object('oprleft',p.oprleft::regtype::text,'oprright',p.oprright::regtype::text,
       'oprresult',p.oprresult::regtype::text,'oprcom',p.oprcom::regoperator::text,'oprnegate',p.oprnegate::regoperator::text,
       'oprcode',p.oprcode::regprocedure::text,'oprrest',p.oprrest::regprocedure::text,'oprjoin',p.oprjoin::regprocedure::text)
   FROM pg_catalog.pg_operator p WHERE p.oid=o.objid)
 WHEN 'pg_catalog.pg_cast'::regclass THEN (
   SELECT jsonb_build_object('source',c.castsource::regtype::text,'target',c.casttarget::regtype::text,
     'function',c.castfunc::regprocedure::text,'context',c.castcontext,'method',c.castmethod)
   FROM pg_catalog.pg_cast c WHERE c.oid=o.objid)
 WHEN 'pg_catalog.pg_am'::regclass THEN (
   SELECT (to_jsonb(a)-'oid') || jsonb_build_object('amhandler',a.amhandler::regprocedure::text) FROM pg_catalog.pg_am a WHERE a.oid=o.objid)
 WHEN 'pg_catalog.pg_opfamily'::regclass THEN (
   SELECT jsonb_build_object('name',p.opfname,'method',a.amname)
   FROM pg_catalog.pg_opfamily p JOIN pg_catalog.pg_am a ON a.oid=p.opfmethod WHERE p.oid=o.objid)
 WHEN 'pg_catalog.pg_opclass'::regclass THEN (
   SELECT jsonb_build_object('name',p.opcname,'method',a.amname,'family',f.opfname,
     'family_schema',n.nspname,'input',p.opcintype::regtype::text,'key',p.opckeytype::regtype::text,'default',p.opcdefault)
   FROM pg_catalog.pg_opclass p JOIN pg_catalog.pg_am a ON a.oid=p.opcmethod
   JOIN pg_catalog.pg_opfamily f ON f.oid=p.opcfamily JOIN pg_catalog.pg_namespace n ON n.oid=f.opfnamespace WHERE p.oid=o.objid)
 WHEN 'pg_catalog.pg_amop'::regclass THEN (
   SELECT jsonb_build_object('family',f.opfname,'family_schema',n.nspname,'method',am.amname,
     'left',a.amoplefttype::regtype::text,'right',a.amoprighttype::regtype::text,
     'strategy',a.amopstrategy,'purpose',a.amoppurpose,'operator',a.amopopr::regoperator::text,
     'sort_family',s.opfname,'sort_schema',sn.nspname)
   FROM pg_catalog.pg_amop a JOIN pg_catalog.pg_opfamily f ON f.oid=a.amopfamily
   JOIN pg_catalog.pg_namespace n ON n.oid=f.opfnamespace JOIN pg_catalog.pg_am am ON am.oid=a.amopmethod
   LEFT JOIN pg_catalog.pg_opfamily s ON s.oid=a.amopsortfamily LEFT JOIN pg_catalog.pg_namespace sn ON sn.oid=s.opfnamespace WHERE a.oid=o.objid)
 WHEN 'pg_catalog.pg_amproc'::regclass THEN (
   SELECT jsonb_build_object('family',f.opfname,'family_schema',n.nspname,
     'left',a.amproclefttype::regtype::text,'right',a.amprocrighttype::regtype::text,
     'number',a.amprocnum,'function',a.amproc::regprocedure::text)
   FROM pg_catalog.pg_amproc a JOIN pg_catalog.pg_opfamily f ON f.oid=a.amprocfamily
   JOIN pg_catalog.pg_namespace n ON n.oid=f.opfnamespace WHERE a.oid=o.objid)
 ELSE NULL END AS definition
 FROM objects o
)
SELECT c.relname, d.objid::bigint, (pg_catalog.pg_identify_object(d.classid,d.objid,0)).identity,
  d.definition::text,
  EXISTS (SELECT 1 FROM members m WHERE m.classid=d.classid AND m.objid=d.objid AND m.objsubid=0)
FROM descriptors d JOIN pg_catalog.pg_class c ON c.oid=d.classid
ORDER BY c.relname,3
LIMIT 4097

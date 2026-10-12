// Run only against a disposable Neo4j Enterprise system database.
// Create the native users agent_graph_ro and agent_graph_attestor securely first.
// The PUBLIC changes affect every user on this disposable instance.

CREATE DATABASE knowledge IF NOT EXISTS WAIT 30 SECONDS;
CREATE DATABASE `private` IF NOT EXISTS WAIT 30 SECONDS;

REVOKE GRANT ACCESS ON HOME DATABASE FROM PUBLIC;
REVOKE GRANT EXECUTE PROCEDURE * ON DBMS FROM PUBLIC;
REVOKE GRANT EXECUTE FUNCTION * ON DBMS FROM PUBLIC;
REVOKE GRANT LOAD ON ALL DATA FROM PUBLIC;

CREATE ROLE agent_graph_reader IF NOT EXISTS;
GRANT ACCESS ON DATABASE knowledge TO agent_graph_reader;
GRANT MATCH {*} ON GRAPH knowledge TO agent_graph_reader;
GRANT SHOW INDEX ON DATABASE knowledge TO agent_graph_reader;
GRANT SHOW CONSTRAINT ON DATABASE knowledge TO agent_graph_reader;
GRANT ROLE agent_graph_reader TO USER agent_graph_ro;

CREATE ROLE agent_graph_catalog IF NOT EXISTS;
GRANT SHOW USER ON DBMS TO agent_graph_catalog;
GRANT SHOW PRIVILEGE ON DBMS TO agent_graph_catalog;
GRANT SHOW AUTH RULE ON DBMS TO agent_graph_catalog;
GRANT SHOW ROLE ON DBMS TO agent_graph_catalog;
GRANT ROLE agent_graph_catalog TO USER agent_graph_attestor;

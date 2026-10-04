// Apply as an administrator to the disposable knowledge graph, not system.
MERGE (d:Neo4jMCPDocument {id: 'neo4j-fixture-doc'})
SET d.title = 'Graph retrieval handbook',
    d.body = 'Graph retrieval connects evidence to a cited source.',
    d.tenant = 'public',
    d.embedding = [0.9, 0.1]
MERGE (s:Neo4jMCPSource {id: 'neo4j-fixture-source'})
SET s.uri = 'fixture://graph-retrieval'
MERGE (d)-[:NEO4J_MCP_CITES]->(s);

MERGE (private:Neo4jMCPDocument {id: 'neo4j-fixture-private'})
SET private.title = 'Private graph retrieval handbook',
    private.body = 'Private graph retrieval must not pass the tenant filter.',
    private.tenant = 'private',
    private.embedding = [1.0, 0.0];

CREATE VECTOR INDEX neo4j_mcp_doc_vector IF NOT EXISTS
FOR (d:Neo4jMCPDocument) ON d.embedding WITH [d.tenant]
OPTIONS {indexConfig: {`vector.dimensions`: 2, `vector.similarity_function`: 'cosine'}};

CREATE FULLTEXT INDEX neo4j_mcp_doc_text IF NOT EXISTS
FOR (d:Neo4jMCPDocument) ON EACH [d.title, d.body];

-- Run with a disposable provisioning identity, not the MCP reader.
-- The vectors schema and pinned vector 0.8.2 extension must already exist.
-- Pass the configured reader role with psql -v reader=agent_pg_reader.
CREATE SCHEMA IF NOT EXISTS reporting;

CREATE TABLE IF NOT EXISTS reporting.incidents (
    id integer PRIMARY KEY,
    service text NOT NULL,
    summary text NOT NULL,
    severity text NOT NULL
);

CREATE TABLE IF NOT EXISTS reporting.incident_embeddings (
    incident_id integer PRIMARY KEY REFERENCES reporting.incidents(id),
    embedding vectors.vector(3) NOT NULL
);

INSERT INTO reporting.incidents (id, service, summary, severity) VALUES
    (101, 'checkout', 'Checkout timeout during payment authorization', 'high'),
    (102, 'checkout', 'Checkout latency after cache expiry', 'medium'),
    (103, 'catalog', 'Catalog indexing delay', 'low')
ON CONFLICT (id) DO UPDATE SET
    service = EXCLUDED.service,
    summary = EXCLUDED.summary,
    severity = EXCLUDED.severity;

INSERT INTO reporting.incident_embeddings (incident_id, embedding) VALUES
    (101, '[1,0,0]'),
    (102, '[0.8,0.6,0]'),
    (103, '[0,1,0]')
ON CONFLICT (incident_id) DO UPDATE SET embedding = EXCLUDED.embedding;

GRANT USAGE ON SCHEMA reporting TO :"reader";
GRANT SELECT ON reporting.incidents, reporting.incident_embeddings TO :"reader";

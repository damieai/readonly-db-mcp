-- Run once as the database administrator in a NEW DISPOSABLE database.
-- Enable ALLOW_SNAPSHOT_ISOLATION on that database before running the test.
-- The harness never receives administrator credentials or runs this script.
CREATE SCHEMA mcp_private AUTHORIZATION dbo;
GO
CREATE SCHEMA reporting AUTHORIZATION dbo;
GO
CREATE TABLE mcp_private.mcp_acceptance_items (
    id int NOT NULL PRIMARY KEY,
    amount int NOT NULL,
    payload nvarchar(max) NOT NULL
);
INSERT mcp_private.mcp_acceptance_items VALUES (1,10,N'["first"]'),(2,20,N'["second"]');
GO
CREATE VIEW reporting.mcp_acceptance_view AS
SELECT id,amount,payload FROM mcp_private.mcp_acceptance_items;
GO
CREATE FUNCTION reporting.mcp_acceptance_scalar(@amount int) RETURNS int
AS BEGIN RETURN @amount+1; END;
GO
CREATE FUNCTION reporting.mcp_acceptance_rows(@minimum int) RETURNS TABLE
AS RETURN (SELECT id,amount FROM mcp_private.mcp_acceptance_items WHERE id>=@minimum);
GO
-- Create a dedicated login/user separately through your normal secret handling.
-- Set its DEFAULT_SCHEMA to reporting. Substitute that user in these grants:
-- GRANT CONNECT TO [acceptance_reader];
-- GRANT SHOWPLAN TO [acceptance_reader];
-- GRANT VIEW DEFINITION TO [acceptance_reader];
-- GRANT SELECT ON OBJECT::sys.sql_expression_dependencies TO [acceptance_reader];
-- GRANT SELECT ON OBJECT::reporting.mcp_acceptance_view TO [acceptance_reader];
-- GRANT EXECUTE ON OBJECT::reporting.mcp_acceptance_scalar TO [acceptance_reader];
-- GRANT SELECT ON OBJECT::reporting.mcp_acceptance_rows TO [acceptance_reader];
-- Do not grant SELECT on mcp_private: the view/function ownership chain is part
-- of the test. Configure allowed_schemas: [reporting].

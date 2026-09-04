package sqlserver

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/your-org/readonly-db-mcp/internal/config"
)

type identity struct {
	version        string
	defaultSchema  string
	deploymentMode string
	readOnly       bool
}

func verifyIdentityAndPrivileges(ctx context.Context, db *sql.DB, cfg *config.TargetConfig) (identity, error) {
	var originalLogin, sessionLogin, databaseUser, database, version, defaultSchema string
	var majorVersion, engineEdition, compatibility, readOnly, snapshotState int
	var sysadmin, databaseOwner sql.NullInt64
	err := db.QueryRowContext(ctx, `
SELECT ORIGINAL_LOGIN(),
       SUSER_SNAME(),
       USER_NAME(),
       DB_NAME(),
       CONVERT(nvarchar(128),SERVERPROPERTY('ProductVersion')),
       CONVERT(int,SERVERPROPERTY('ProductMajorVersion')),
       CONVERT(int,SERVERPROPERTY('EngineEdition')),
       d.compatibility_level,
       CASE WHEN d.is_read_only=1 OR CONVERT(nvarchar(60),DATABASEPROPERTYEX(DB_NAME(),'Updateability'))='READ_ONLY' THEN 1 ELSE 0 END,
       d.snapshot_isolation_state,
       COALESCE(principal.default_schema_name,N'dbo'),
       IS_SRVROLEMEMBER(N'sysadmin'),
       IS_ROLEMEMBER(N'db_owner')
FROM sys.databases AS d
LEFT JOIN sys.database_principals AS principal ON principal.name=USER_NAME()
WHERE d.name=DB_NAME()`).Scan(
		&originalLogin, &sessionLogin, &databaseUser, &database, &version,
		&majorVersion, &engineEdition, &compatibility, &readOnly,
		&snapshotState, &defaultSchema, &sysadmin, &databaseOwner,
	)
	if err != nil {
		return identity{}, errors.New("inspect SQL Server identity")
	}
	if !strings.EqualFold(originalLogin, cfg.Username) ||
		!strings.EqualFold(sessionLogin, cfg.Username) ||
		!strings.EqualFold(database, cfg.Database) {
		return identity{}, errors.New("connected SQL Server identity does not match configuration")
	}
	// A login-to-user mapping commonly uses different names. Effective database
	// permissions below are authoritative; only privileged implicit users are
	// rejected here.
	if strings.EqualFold(databaseUser, "dbo") || strings.EqualFold(databaseUser, "guest") ||
		strings.EqualFold(databaseUser, "sys") || strings.EqualFold(databaseUser, "INFORMATION_SCHEMA") {
		return identity{}, errors.New("SQL Server login resolved to a privileged or implicit database user")
	}
	if sysadmin.Int64 == 1 || databaseOwner.Int64 == 1 {
		return identity{}, errors.New("SQL Server login must not be sysadmin or db_owner")
	}
	deploymentMode := "sql-server"
	if engineEdition == 5 {
		deploymentMode = "azure-sql-database"
	} else if engineEdition == 8 {
		deploymentMode = "azure-sql-managed-instance"
	}
	if engineEdition == 5 || engineEdition == 8 {
		if compatibility < 150 || compatibility > 170 {
			return identity{}, fmt.Errorf("SQL Server compatibility level %d is not supported", compatibility)
		}
	} else if majorVersion < 15 || majorVersion > 17 {
		return identity{}, fmt.Errorf("SQL Server major version %d is not supported", majorVersion)
	}
	if _, allowed := lowerSet(cfg.AllowedSchemas)[strings.ToLower(defaultSchema)]; !allowed {
		return identity{}, errors.New("SQL Server user's default schema must be in allowed_schemas")
	}
	if cfg.SQLServer.RequireReadOnlyReplica && readOnly != 1 {
		return identity{}, errors.New("SQL Server database is not a read-only replica")
	}
	if cfg.SQLServer.RequireSnapshot == nil || !*cfg.SQLServer.RequireSnapshot || snapshotState != 1 {
		return identity{}, errors.New("SQL Server ALLOW_SNAPSHOT_ISOLATION must be ON")
	}
	if err := verifyPermissionSets(ctx, db, cfg); err != nil {
		return identity{}, err
	}
	return identity{version: version, defaultSchema: defaultSchema, deploymentMode: deploymentMode, readOnly: readOnly == 1}, nil
}

func verifyPermissionSets(ctx context.Context, db *sql.DB, cfg *config.TargetConfig) error {
	serverAllowed := stringSet(
		"connect sql", "view any database", "view any definition",
		"view server state", "view server performance state", "view server security state",
		"view any security definition", "view any performance definition",
		"view any cryptographically secured definition",
	)
	if err := verifyFlatPermissions(ctx, db, `SELECT permission_name FROM sys.fn_my_permissions(NULL,N'SERVER')`, serverAllowed, "server"); err != nil {
		return err
	}
	databaseAllowed := stringSet(
		"connect", "select", "showplan", "view definition", "view database state",
		"view database performance state", "view database security state", "view change tracking",
		"view security definition", "view performance definition", "view cryptographically secured definition",
		"view any column encryption key definition", "view any column master key definition",
		"view any sensitivity classification",
	)
	permissions, err := currentPermissions(ctx, db, `SELECT permission_name FROM sys.fn_my_permissions(NULL,N'DATABASE')`)
	if err != nil {
		return errors.New("inspect SQL Server database permissions")
	}
	if _, ok := permissions["connect"]; !ok {
		return errors.New("SQL Server user lacks CONNECT permission")
	}
	if _, ok := permissions["showplan"]; !ok {
		return errors.New("SQL Server user lacks mandatory SHOWPLAN permission")
	}
	for permission := range permissions {
		if _, ok := databaseAllowed[permission]; !ok {
			return fmt.Errorf("SQL Server database permission %q exceeds the read-only profile", permission)
		}
	}
	var grantOptions int
	if err := db.QueryRowContext(ctx, `
SELECT (SELECT COUNT(*)
        FROM sys.server_permissions AS permission_value
        JOIN sys.login_token AS token_value ON token_value.principal_id=permission_value.grantee_principal_id
        WHERE permission_value.state=N'W')+
       (SELECT COUNT(*)
        FROM sys.database_permissions AS permission_value
        JOIN sys.user_token AS token_value ON token_value.principal_id=permission_value.grantee_principal_id
        WHERE permission_value.state=N'W')`).Scan(&grantOptions); err != nil {
		return errors.New("inspect SQL Server grant options")
	}
	if grantOptions != 0 {
		return errors.New("SQL Server user or one of its roles has WITH GRANT OPTION")
	}

	allowedSchemas := lowerSet(cfg.AllowedSchemas)
	deniedTables := lowerSet(cfg.DeniedTables)
	rows, err := db.QueryContext(ctx, `
SELECT schema_value.name,
       HAS_PERMS_BY_NAME(QUOTENAME(schema_value.name),N'SCHEMA',N'SELECT'),
       HAS_PERMS_BY_NAME(QUOTENAME(schema_value.name),N'SCHEMA',N'INSERT'),
       HAS_PERMS_BY_NAME(QUOTENAME(schema_value.name),N'SCHEMA',N'UPDATE'),
       HAS_PERMS_BY_NAME(QUOTENAME(schema_value.name),N'SCHEMA',N'DELETE'),
       HAS_PERMS_BY_NAME(QUOTENAME(schema_value.name),N'SCHEMA',N'EXECUTE'),
       HAS_PERMS_BY_NAME(QUOTENAME(schema_value.name),N'SCHEMA',N'ALTER'),
       HAS_PERMS_BY_NAME(QUOTENAME(schema_value.name),N'SCHEMA',N'CONTROL'),
       HAS_PERMS_BY_NAME(QUOTENAME(schema_value.name),N'SCHEMA',N'TAKE OWNERSHIP')
FROM sys.schemas AS schema_value
WHERE schema_value.name NOT IN (N'sys',N'INFORMATION_SCHEMA')`)
	if err != nil {
		return errors.New("inspect SQL Server schema permissions")
	}
	for rows.Next() {
		var schema string
		var selectPermission, insertPermission, updatePermission, deletePermission int
		var executePermission, alterPermission, controlPermission, ownershipPermission int
		if err := rows.Scan(&schema, &selectPermission, &insertPermission, &updatePermission, &deletePermission, &executePermission, &alterPermission, &controlPermission, &ownershipPermission); err != nil {
			rows.Close()
			return errors.New("read SQL Server schema permissions")
		}
		_, allowed := allowedSchemas[strings.ToLower(schema)]
		if insertPermission == 1 || updatePermission == 1 || deletePermission == 1 || executePermission == 1 || alterPermission == 1 || controlPermission == 1 || ownershipPermission == 1 || (!allowed && selectPermission == 1) {
			rows.Close()
			return fmt.Errorf("SQL Server schema %q permissions exceed the configured read-only scope", schema)
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return errors.New("read SQL Server schema permissions")
	}
	rows.Close()

	rows, err = db.QueryContext(ctx, `
SELECT schema_value.name,
       object_value.name,
       object_value.type,
       HAS_PERMS_BY_NAME(QUOTENAME(schema_value.name)+N'.'+QUOTENAME(object_value.name),N'OBJECT',N'SELECT'),
       HAS_PERMS_BY_NAME(QUOTENAME(schema_value.name)+N'.'+QUOTENAME(object_value.name),N'OBJECT',N'INSERT'),
       HAS_PERMS_BY_NAME(QUOTENAME(schema_value.name)+N'.'+QUOTENAME(object_value.name),N'OBJECT',N'UPDATE'),
       HAS_PERMS_BY_NAME(QUOTENAME(schema_value.name)+N'.'+QUOTENAME(object_value.name),N'OBJECT',N'DELETE'),
       HAS_PERMS_BY_NAME(QUOTENAME(schema_value.name)+N'.'+QUOTENAME(object_value.name),N'OBJECT',N'EXECUTE'),
		HAS_PERMS_BY_NAME(QUOTENAME(schema_value.name)+N'.'+QUOTENAME(object_value.name),N'OBJECT',N'ALTER'),
		HAS_PERMS_BY_NAME(QUOTENAME(schema_value.name)+N'.'+QUOTENAME(object_value.name),N'OBJECT',N'CONTROL'),
		HAS_PERMS_BY_NAME(QUOTENAME(schema_value.name)+N'.'+QUOTENAME(object_value.name),N'OBJECT',N'TAKE OWNERSHIP'),
		HAS_PERMS_BY_NAME(QUOTENAME(schema_value.name)+N'.'+QUOTENAME(object_value.name),N'OBJECT',N'REFERENCES')
FROM sys.objects AS object_value
JOIN sys.schemas AS schema_value ON schema_value.schema_id=object_value.schema_id
WHERE object_value.is_ms_shipped=0`)
	if err != nil {
		return errors.New("inspect SQL Server object permissions")
	}
	for rows.Next() {
		var schema, name, objectType string
		var selectPermission, insertPermission, updatePermission, deletePermission int
		var executePermission, alterPermission, controlPermission, ownershipPermission, referencesPermission int
		if err := rows.Scan(&schema, &name, &objectType, &selectPermission, &insertPermission, &updatePermission, &deletePermission, &executePermission, &alterPermission, &controlPermission, &ownershipPermission, &referencesPermission); err != nil {
			rows.Close()
			return errors.New("read SQL Server object permissions")
		}
		qualified := strings.ToLower(schema + "." + name)
		_, allowed := allowedSchemas[strings.ToLower(schema)]
		_, deniedByName := deniedTables[strings.ToLower(name)]
		_, deniedQualified := deniedTables[qualified]
		if insertPermission == 1 || updatePermission == 1 || deletePermission == 1 || executePermission == 1 || alterPermission == 1 || controlPermission == 1 || ownershipPermission == 1 || referencesPermission == 1 {
			rows.Close()
			return fmt.Errorf("SQL Server object %q has a data-changing or executable permission", schema+"."+name)
		}
		if selectPermission == 1 && (!allowed || deniedByName || deniedQualified) {
			rows.Close()
			return fmt.Errorf("SQL Server object %q is selectable outside the configured scope", schema+"."+name)
		}
		// SQL CLR functions can perform effects outside the database while
		// appearing under SELECT. Inline and T-SQL functions remain available.
		if selectPermission == 1 && (objectType == "FS" || objectType == "FT") {
			rows.Close()
			return fmt.Errorf("SQL CLR function %q is not accepted as side-effect-free", schema+"."+name)
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return errors.New("read SQL Server object permissions")
	}
	rows.Close()

	rows, err = db.QueryContext(ctx, `
SELECT schema_value.name,
       object_value.name,
       column_value.name,
		HAS_PERMS_BY_NAME(QUOTENAME(schema_value.name)+N'.'+QUOTENAME(object_value.name),N'OBJECT',N'SELECT',column_value.name,N'COLUMN'),
		HAS_PERMS_BY_NAME(QUOTENAME(schema_value.name)+N'.'+QUOTENAME(object_value.name),N'OBJECT',N'INSERT',column_value.name,N'COLUMN'),
		HAS_PERMS_BY_NAME(QUOTENAME(schema_value.name)+N'.'+QUOTENAME(object_value.name),N'OBJECT',N'UPDATE',column_value.name,N'COLUMN'),
		HAS_PERMS_BY_NAME(QUOTENAME(schema_value.name)+N'.'+QUOTENAME(object_value.name),N'OBJECT',N'REFERENCES',column_value.name,N'COLUMN')
FROM sys.columns AS column_value
JOIN sys.objects AS object_value ON object_value.object_id=column_value.object_id
JOIN sys.schemas AS schema_value ON schema_value.schema_id=object_value.schema_id
WHERE object_value.is_ms_shipped=0`)
	if err != nil {
		return errors.New("inspect SQL Server column permissions")
	}
	for rows.Next() {
		var schema, table, column string
		var selectPermission, insertPermission, updatePermission, referencesPermission int
		if err := rows.Scan(&schema, &table, &column, &selectPermission, &insertPermission, &updatePermission, &referencesPermission); err != nil {
			rows.Close()
			return errors.New("read SQL Server column permissions")
		}
		qualified := strings.ToLower(schema + "." + table)
		_, allowed := allowedSchemas[strings.ToLower(schema)]
		_, deniedByName := deniedTables[strings.ToLower(table)]
		_, deniedQualified := deniedTables[qualified]
		if insertPermission == 1 || updatePermission == 1 || referencesPermission == 1 {
			rows.Close()
			return fmt.Errorf("SQL Server column %q has a data-changing permission", schema+"."+table+"."+column)
		}
		if selectPermission == 1 && (!allowed || deniedByName || deniedQualified) {
			rows.Close()
			return fmt.Errorf("SQL Server column %q is selectable outside the configured scope", schema+"."+table+"."+column)
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return errors.New("read SQL Server column permissions")
	}
	rows.Close()

	var owned int
	if err := db.QueryRowContext(ctx, `
SELECT (SELECT COUNT(*) FROM sys.schemas WHERE principal_id=DATABASE_PRINCIPAL_ID())+
       (SELECT COUNT(*) FROM sys.objects WHERE principal_id=DATABASE_PRINCIPAL_ID())+
       (SELECT COUNT(*) FROM sys.assemblies WHERE principal_id=DATABASE_PRINCIPAL_ID() AND is_user_defined=1)`).Scan(&owned); err != nil {
		return errors.New("inspect SQL Server ownership")
	}
	if owned != 0 {
		return errors.New("SQL Server user must not own schemas, objects, or assemblies")
	}
	return nil
}

func verifyFlatPermissions(ctx context.Context, db *sql.DB, query string, allowed map[string]struct{}, level string) error {
	permissions, err := currentPermissions(ctx, db, query)
	if err != nil {
		return fmt.Errorf("inspect SQL Server %s permissions", level)
	}
	for permission := range permissions {
		if _, ok := allowed[permission]; !ok {
			return fmt.Errorf("SQL Server %s permission %q exceeds the read-only profile", level, permission)
		}
	}
	return nil
}

func currentPermissions(ctx context.Context, db *sql.DB, query string) (map[string]struct{}, error) {
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[string]struct{})
	for rows.Next() {
		var permission string
		if err := rows.Scan(&permission); err != nil {
			return nil, err
		}
		result[strings.ToLower(permission)] = struct{}{}
	}
	return result, rows.Err()
}

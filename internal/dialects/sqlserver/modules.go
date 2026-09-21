package sqlserver

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

const maxModuleObjects = 10000

func (t *Target) openCatalog(ctx context.Context) error {
	t.catalogDB = t.db
	if attestor := t.cfg.SQLServer.Attestor; attestor != nil {
		cfg := *t.cfg
		cfg.Username, cfg.PasswordFile, cfg.PasswordEnv = attestor.Username, attestor.PasswordFile, attestor.PasswordEnv
		// A separate pool can only receive fixed catalog statements. Its capacity
		// is explicitly one connection, included in the target capacity review.
		cfg.Connection.MaxOpen, cfg.Connection.MaxIdle = 1, 1
		if t.cfg.Connection.MaxIdle == 0 {
			cfg.Connection.MaxIdle = 0
		}
		password, err := cfg.Password()
		if err != nil {
			return errors.New("read SQL Server attestor credentials")
		}
		db, err := openDB(&cfg, password)
		password = ""
		if err != nil {
			return errors.New("open SQL Server attestor")
		}
		if _, err := verifyIdentityAndPrivileges(ctx, db, &cfg); err != nil {
			_ = db.Close()
			return fmt.Errorf("SQL Server attestor: %w", err)
		}
		t.catalogDB = db
	}
	var visible int
	if err := t.catalogDB.QueryRowContext(ctx, catalogVisibilitySQL).Scan(&visible); err != nil || visible != 1 {
		if t.catalogDB != t.db {
			_ = t.catalogDB.Close()
		}
		return errors.New("SQL Server module attestation requires VIEW DEFINITION and SELECT on sys.sql_expression_dependencies through the runtime identity or a read-only attestor")
	}
	return nil
}

func (t *Target) attestAccessibleModules(ctx context.Context) error {
	rows, err := t.db.QueryContext(ctx, `SELECT TOP (10001) s.name,o.name FROM sys.objects o JOIN sys.schemas s ON s.schema_id=o.schema_id WHERE o.is_ms_shipped=0 AND (HAS_PERMS_BY_NAME(QUOTENAME(s.name)+N'.'+QUOTENAME(o.name),N'OBJECT',N'SELECT')=1 OR (o.type IN ('FN','IF','TF') AND HAS_PERMS_BY_NAME(QUOTENAME(s.name)+N'.'+QUOTENAME(o.name),N'OBJECT',N'EXECUTE')=1))`)
	if err != nil {
		return errors.New("inspect SQL Server executable entry objects")
	}
	var refs []parserReference
	for rows.Next() {
		var schema, name string
		if err := rows.Scan(&schema, &name); err != nil {
			rows.Close()
			return errors.New("read SQL Server entry inventory")
		}
		refs = append(refs, parserReference{Parts: []string{schema, name}, Kind: "relation"})
		if len(refs) > maxModuleObjects {
			rows.Close()
			return errors.New("SQL Server entry inventory exceeds limit")
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return errors.New("read SQL Server entry inventory")
	}
	return t.verifyModuleReferences(ctx, refs)
}

type moduleObject struct {
	id                                      int
	schema, name, kind, definition, synonym string
	unsafe                                  bool
	dependencies                            []int
}

// Catalog queries contain no caller SQL. Definitions and dependencies are
// inspected under VIEW DEFINITION, including tables' computed columns and RLS
// predicates. Every execution rechecks its closure rather than trusting a plan
// to expose non-inlined functions or eliminated branches.
func (t *Target) verifyModuleReferences(ctx context.Context, refs []parserReference) error {
	if len(refs) == 0 {
		return nil
	}
	objects, names, err := loadModuleGraph(ctx, t.catalogDB)
	if err != nil {
		return err
	}
	visited := map[int]bool{}
	var visit func(int) error
	visit = func(id int) error {
		if visited[id] {
			return nil
		}
		object := objects[id]
		if object == nil || object.unsafe {
			return errors.New("SQL Server dependency has an unproven executable or external capability")
		}
		visited[id] = true
		var parsedRefs []parserReference
		switch object.kind {
		case "U":
		case "V", "FN", "IF", "TF":
			if object.definition == "" {
				return errors.New("SQL Server module definition is hidden or encrypted")
			}
			proof, err := t.parser.Parse(ctx, object.definition, t.compatibility, true)
			if err != nil {
				return errors.New("SQL Server module definition is not proven read-only")
			}
			parsedRefs = proof.References
		case "SN":
			proof, err := t.parser.Parse(ctx, "SELECT * FROM "+object.synonym, t.compatibility, false)
			if err != nil || len(proof.References) != 1 {
				return errors.New("SQL Server synonym does not resolve to one local object")
			}
			parsedRefs = proof.References
		default:
			return errors.New("SQL Server dependency type is not a read-only T-SQL object")
		}
		for _, reference := range parsedRefs {
			if len(reference.Parts) == 1 && object.kind != "SN" {
				// The native dependency IDs capture binding in the module's
				// context, which can differ from the runtime user's schema.
				// Inspect every matching edge if multiple schemas share a name.
				resolved := false
				for _, child := range object.dependencies {
					dependency := objects[child]
					if dependency != nil && strings.EqualFold(dependency.name, reference.Parts[0]) {
						if err := visit(child); err != nil {
							return err
						}
						resolved = true
					}
				}
				if !resolved {
					return errors.New("SQL Server module has an unresolved native dependency")
				}
				continue
			}
			schema, name, err := t.referenceName(reference)
			if err != nil {
				return err
			}
			child, ok := names[strings.ToLower(schema+"\x00"+name)]
			if !ok {
				return errors.New("SQL Server module has an unresolved dependency")
			}
			if err := visit(child); err != nil {
				return err
			}
		}
		for _, child := range object.dependencies {
			if err := visit(child); err != nil {
				return err
			}
		}
		return nil
	}
	for _, ref := range refs {
		schema, name, err := t.referenceName(ref)
		if err != nil {
			return err
		}
		id, exists := names[strings.ToLower(schema+"\x00"+name)]
		if !exists {
			return errors.New("SQL Server entry object is missing from the complete catalog")
		}
		if err := visit(id); err != nil {
			return err
		}
	}
	return nil
}

const catalogVisibilitySQL = `SELECT HAS_PERMS_BY_NAME(DB_NAME(),N'DATABASE',N'VIEW DEFINITION') & HAS_PERMS_BY_NAME(N'sys.sql_expression_dependencies',N'OBJECT',N'SELECT')`

func loadModuleGraph(ctx context.Context, db *sql.DB) (map[int]*moduleObject, map[string]int, error) {
	if db == nil {
		return nil, nil, errors.New("SQL Server catalog identity is unavailable")
	}
	var visible int
	if err := db.QueryRowContext(ctx, catalogVisibilitySQL).Scan(&visible); err != nil || visible != 1 {
		return nil, nil, errors.New("SQL Server catalog visibility proof is stale")
	}
	rows, err := db.QueryContext(ctx, `
SELECT TOP (10001) o.object_id,s.name,o.name,o.type,
 CASE WHEN DATALENGTH(m.definition)<=2097152 THEN COALESCE(m.definition,N'') ELSE N'' END,
 COALESCE(sn.base_object_name,N''),
 CASE WHEN m.execute_as_principal_id IS NOT NULL
 OR EXISTS(SELECT 1 FROM sys.crypt_properties cp WHERE cp.major_id=o.object_id AND cp.class=1)
 OR EXISTS(SELECT 1 FROM sys.external_tables e WHERE e.object_id=o.object_id)
 OR EXISTS(SELECT 1 FROM sys.columns c JOIN sys.types ty ON ty.user_type_id=c.user_type_id WHERE c.object_id=o.object_id AND ty.is_assembly_type=1 AND ty.is_user_defined=1)
 OR EXISTS(SELECT 1 FROM sys.parameters p JOIN sys.types ty ON ty.user_type_id=p.user_type_id WHERE p.object_id=o.object_id AND ty.is_assembly_type=1 AND ty.is_user_defined=1)
 THEN 1 ELSE 0 END
FROM sys.objects o JOIN sys.schemas s ON s.schema_id=o.schema_id
LEFT JOIN sys.sql_modules m ON m.object_id=o.object_id
LEFT JOIN sys.synonyms sn ON sn.object_id=o.object_id
WHERE o.is_ms_shipped=0`)
	if err != nil {
		return nil, nil, errors.New("inspect SQL Server module definitions")
	}
	objects := map[int]*moduleObject{}
	names := map[string]int{}
	definitionBytes := 0
	for rows.Next() {
		object := &moduleObject{}
		if err := rows.Scan(&object.id, &object.schema, &object.name, &object.kind, &object.definition, &object.synonym, &object.unsafe); err != nil {
			rows.Close()
			return nil, nil, errors.New("read SQL Server module definitions")
		}
		object.kind = strings.TrimSpace(object.kind)
		definitionBytes += len(object.definition)
		if definitionBytes > 16<<20 {
			rows.Close()
			return nil, nil, errors.New("SQL Server module definitions exceed the 16 MiB proof budget")
		}
		key := strings.ToLower(object.schema + "\x00" + object.name)
		if _, duplicate := names[key]; duplicate {
			rows.Close()
			return nil, nil, errors.New("SQL Server catalog has ambiguous identifier casing")
		}
		objects[object.id], names[key] = object, object.id
		if len(objects) > maxModuleObjects {
			rows.Close()
			return nil, nil, errors.New("SQL Server module inventory exceeds limit")
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, nil, errors.New("read SQL Server module definitions")
	}
	rows, err = db.QueryContext(ctx, `
SELECT TOP (100001) referencing_id,referenced_id,
 CASE WHEN referenced_server_name IS NOT NULL OR (referenced_database_name IS NOT NULL AND referenced_database_name<>DB_NAME()) OR is_caller_dependent=1 OR is_ambiguous=1 OR referenced_id IS NULL
 OR (referenced_class=6 AND NOT EXISTS (
   SELECT 1 FROM sys.types ty WHERE ty.user_type_id=d.referenced_id
   AND (ty.is_assembly_type=0 OR ty.is_user_defined=0)
   AND NOT EXISTS (SELECT 1 FROM sys.table_types tt JOIN sys.columns c ON c.object_id=tt.type_table_object_id JOIN sys.types ct ON ct.user_type_id=c.user_type_id
     WHERE tt.user_type_id=ty.user_type_id AND ct.is_assembly_type=1 AND ct.is_user_defined=1)))
 THEN 1 ELSE 0 END, referenced_class
FROM sys.sql_expression_dependencies d WHERE referencing_class=1
UNION ALL
SELECT target_object_id,referenced_id,
 CASE WHEN referenced_id IS NULL THEN 1 ELSE 0 END, referenced_class
FROM sys.security_predicates p JOIN sys.sql_expression_dependencies d ON d.referencing_id=p.object_id AND d.referenced_class=1`)
	if err != nil {
		return nil, nil, errors.New("inspect SQL Server transitive module dependencies")
	}
	count := 0
	for rows.Next() {
		var from, class int
		var to sql.NullInt64
		var unsafe bool
		if err := rows.Scan(&from, &to, &unsafe, &class); err != nil {
			rows.Close()
			return nil, nil, errors.New("read SQL Server module dependencies")
		}
		count++
		if count > 100000 {
			rows.Close()
			return nil, nil, errors.New("SQL Server dependency graph exceeds limit")
		}
		if object := objects[from]; object != nil {
			object.unsafe = object.unsafe || unsafe
			switch class {
			case 1:
				if to.Valid {
					object.dependencies = append(object.dependencies, int(to.Int64))
				}
			case 6, 7, 10, 21:
				// Type/index/XML collection/partition function IDs are not
				// object IDs. Native CLR type capabilities are checked above;
				// the remaining dependencies describe declarative metadata.
			default:
				object.unsafe = true
			}
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, nil, errors.New("read SQL Server module dependencies")
	}
	return objects, names, nil
}

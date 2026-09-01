package mysql

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/your-org/readonly-db-mcp/internal/core"
	"vitess.io/vitess/go/vt/sqlparser"
)

var systemSchemas = map[string]struct{}{
	"information_schema": {},
	"mysql":              {},
	"performance_schema": {},
	"sys":                {},
}

var volatileSQL = regexp.MustCompile(`(?i)\b(rand|random_bytes|uuid|uuid_short|now|sysdate|curdate|current_date|curtime|current_time|current_timestamp|localtime|localtimestamp|connection_id|last_insert_id)\b`)

// safeGenericFunctions contains side-effect-free built-ins represented by
// Vitess as FuncExpr. Aggregate, window, JSON and other typed AST functions are
// accepted independently. Unknown generic functions fail closed because MySQL
// loadable functions look identical to built-ins at the SQL surface.
var safeGenericFunctions = stringSet(
	"abs", "acos", "adddate", "ascii", "asin", "atan", "atan2",
	"bin", "bit_count", "ceil", "ceiling", "char", "char_length",
	"character_length", "coalesce", "concat", "concat_ws", "conv",
	"convert_tz", "cos", "cot", "crc32", "date", "datediff", "date_format",
	"date_add", "date_sub", "day", "dayname", "dayofmonth", "dayofweek",
	"dayofyear", "degrees", "elt", "exp", "export_set", "field",
	"find_in_set", "floor", "format", "from_base64", "from_days",
	"from_unixtime", "greatest", "hex", "hour", "if", "ifnull",
	"inet_aton", "inet_ntoa", "instr", "isnull", "lcase", "least",
	"left", "length", "ln", "locate", "log", "log10", "log2", "lower",
	"lpad", "ltrim", "makedate", "maketime", "md5", "microsecond",
	"mid", "minute", "mod", "month", "monthname", "nullif", "oct",
	"octet_length", "ord", "period_add", "period_diff", "pi", "position",
	"pow", "power", "quarter", "quote", "radians", "rand", "repeat",
	"replace", "reverse", "right", "round", "rpad", "rtrim", "second",
	"sec_to_time", "sha", "sha1", "sha2", "sign", "sin", "space",
	"sqrt", "strcmp", "str_to_date", "substr", "substring",
	"substring_index", "tan", "time", "timediff", "timestamp",
	"timestampadd", "timestampdiff", "time_format", "time_to_sec", "to_base64",
	"to_days", "to_seconds", "trim", "truncate", "ucase", "unhex",
	"unix_timestamp", "upper", "uuid_to_bin", "bin_to_uuid", "week",
	"weekday", "weekofyear", "year", "yearweek",
)

type Policy struct {
	parser         *sqlparser.Parser
	defaultSchema  string
	allowedSchemas map[string]struct{}
	deniedTables   map[string]struct{}
	maxSQLBytes    int
}

func NewPolicy(defaultSchema string, allowedSchemas, deniedTables []string, maxSQLBytes int) *Policy {
	return &Policy{
		parser:         sqlparser.NewTestParser(),
		defaultSchema:  strings.ToLower(defaultSchema),
		allowedSchemas: lowerSet(allowedSchemas),
		deniedTables:   lowerSet(deniedTables),
		maxSQLBytes:    maxSQLBytes,
	}
}

func (p *Policy) Validate(query string) (*core.Validation, error) {
	trimmed := strings.TrimSpace(query)
	fingerprint := hashSQL(trimmed)
	if trimmed == "" {
		return nil, fmt.Errorf("query is empty")
	}
	if len(trimmed) > p.maxSQLBytes {
		return nil, fmt.Errorf("query exceeds the configured SQL size limit")
	}
	if strings.Contains(trimmed, "/*!") || strings.Contains(trimmed, "/*+") {
		return nil, fmt.Errorf("executable comments and optimizer hints are not allowed")
	}

	statement, err := p.parser.Parse(trimmed)
	if err != nil {
		return nil, fmt.Errorf("query is not one supported MySQL statement")
	}
	switch statement.(type) {
	case *sqlparser.Select, *sqlparser.Union:
	default:
		return nil, fmt.Errorf("only SELECT and UNION queries are allowed")
	}

	cteNames := make(map[string]struct{})
	if err := sqlparser.Walk(func(node sqlparser.SQLNode) (bool, error) {
		if cte, ok := node.(*sqlparser.CommonTableExpr); ok {
			cteNames[strings.ToLower(cte.ID.String())] = struct{}{}
		}
		return true, nil
	}, statement); err != nil {
		return nil, fmt.Errorf("inspect common table expressions: %w", err)
	}

	tableSet := make(map[string]struct{})
	err = sqlparser.Walk(func(node sqlparser.SQLNode) (bool, error) {
		switch typed := node.(type) {
		case *sqlparser.Select:
			if typed.Into != nil {
				return false, fmt.Errorf("SELECT INTO is not allowed")
			}
			if typed.Lock != sqlparser.NoLock {
				return false, fmt.Errorf("locking SELECT queries are not allowed")
			}
		case *sqlparser.Union:
			if typed.Into != nil {
				return false, fmt.Errorf("SELECT INTO is not allowed")
			}
			if typed.Lock != sqlparser.NoLock {
				return false, fmt.Errorf("locking SELECT queries are not allowed")
			}
		case *sqlparser.Variable:
			return false, fmt.Errorf("session and user variables are not allowed")
		case *sqlparser.LockingFunc:
			return false, fmt.Errorf("advisory lock functions are not allowed")
		case *sqlparser.GTIDFuncExpr:
			return false, fmt.Errorf("GTID wait functions are not allowed")
		case *sqlparser.FuncExpr:
			if !typed.Qualifier.IsEmpty() {
				return false, fmt.Errorf("qualified or stored functions are not allowed")
			}
			name := typed.Name.Lowered()
			if _, ok := safeGenericFunctions[name]; !ok {
				return false, fmt.Errorf("function %q is not in the audited function set", name)
			}
		case *sqlparser.AliasedTableExpr:
			table, ok := typed.Expr.(sqlparser.TableName)
			if !ok {
				return true, nil
			}
			name := strings.ToLower(table.Name.String())
			schema := strings.ToLower(table.Qualifier.String())
			if schema == "" {
				if _, isCTE := cteNames[name]; isCTE {
					return true, nil
				}
				schema = p.defaultSchema
			}
			if _, system := systemSchemas[schema]; system {
				return false, fmt.Errorf("system schema %q is not available to free-form queries", schema)
			}
			if _, allowed := p.allowedSchemas[schema]; !allowed {
				return false, fmt.Errorf("schema %q is outside the selected target", schema)
			}
			qualified := schema + "." + name
			if _, denied := p.deniedTables[name]; denied {
				return false, fmt.Errorf("table %q is denied by target policy", name)
			}
			if _, denied := p.deniedTables[qualified]; denied {
				return false, fmt.Errorf("table %q is denied by target policy", qualified)
			}
			tableSet[qualified] = struct{}{}
		}
		return true, nil
	}, statement)
	if err != nil {
		return nil, err
	}

	tables := make([]string, 0, len(tableSet))
	for table := range tableSet {
		tables = append(tables, table)
	}
	sort.Strings(tables)
	return &core.Validation{Fingerprint: fingerprint, Tables: tables, Cacheable: !volatileSQL.MatchString(trimmed)}, nil
}

func hashSQL(query string) string {
	sum := sha256.Sum256([]byte(query))
	return hex.EncodeToString(sum[:12])
}

func lowerSet(values []string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		result[strings.ToLower(value)] = struct{}{}
	}
	return result
}

func stringSet(values ...string) map[string]struct{} {
	return lowerSet(values)
}

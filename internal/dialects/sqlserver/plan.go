package sqlserver

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

const maxShowPlanBytes = 16 << 20

// showPlan proves what SQL Server compiled, rather than treating a client-side
// token scan as a T-SQL parser. SHOWPLAN_XML compiles the statement but never
// executes it. It is mandatory for both Query and Explain.
func (t *Target) showPlan(ctx context.Context, query string, args []any) (plan string, err error) {
	conn, err := t.db.Conn(ctx)
	if err != nil {
		return "", sanitize(err)
	}
	defer conn.Close()

	if _, err = conn.ExecContext(ctx, "SET SHOWPLAN_XML ON"); err != nil {
		return "", errors.New("SQL Server SHOWPLAN permission is required")
	}
	showPlanEnabled := true
	defer func() {
		if !showPlanEnabled {
			return
		}
		// SessionInitSQL causes go-mssqldb to reset pooled sessions as well;
		// this explicit OFF keeps the normal success path clean immediately.
		restoreTimeout := t.cfg.Connection.WriteTimeout
		if restoreTimeout <= 0 {
			restoreTimeout = 3 * time.Second
		}
		offCtx, cancel := context.WithTimeout(context.Background(), restoreTimeout)
		defer cancel()
		if _, offErr := conn.ExecContext(offCtx, "SET SHOWPLAN_XML OFF"); offErr != nil && err == nil {
			err = errors.New("failed to restore SQL Server SHOWPLAN session state")
		}
	}()

	rows, queryErr := conn.QueryContext(ctx, query, args...)
	if queryErr != nil {
		return "", sanitize(queryErr)
	}
	defer rows.Close()
	var plans []string
	for rows.Next() {
		var raw any
		if scanErr := rows.Scan(&raw); scanErr != nil {
			return "", errors.New("read SQL Server SHOWPLAN result")
		}
		var value string
		switch x := raw.(type) {
		case string:
			value = x
		case []byte:
			value = string(x)
		default:
			return "", errors.New("SQL Server returned an unexpected SHOWPLAN value")
		}
		if len(value) > maxShowPlanBytes {
			return "", errors.New("SQL Server SHOWPLAN exceeds the safety limit")
		}
		plans = append(plans, value)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return "", sanitize(rowsErr)
	}
	if len(plans) != 1 {
		return "", errors.New("SQL Server must compile exactly one statement")
	}
	if err = validateShowPlan(plans[0], t.cfg.Database, t.denied); err != nil {
		return "", err
	}
	rows.Close()
	if _, err = conn.ExecContext(ctx, "SET SHOWPLAN_XML OFF"); err != nil {
		return "", errors.New("failed to restore SQL Server SHOWPLAN session state")
	}
	showPlanEnabled = false
	return plans[0], nil
}

func validateShowPlan(plan, database string, denied map[string]struct{}) error {
	if strings.TrimSpace(plan) == "" || len(plan) > maxShowPlanBytes {
		return errors.New("SQL Server returned an invalid SHOWPLAN")
	}
	decoder := xml.NewDecoder(io.LimitReader(strings.NewReader(plan), maxShowPlanBytes+1))
	stack := make([]string, 0, 16)
	statements := 0
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return errors.New("SQL Server returned malformed SHOWPLAN XML")
		}
		switch value := token.(type) {
		case xml.StartElement:
			parent := ""
			if len(stack) > 0 {
				parent = stack[len(stack)-1]
			}
			name := value.Name.Local
			if parent == "Statements" && strings.HasPrefix(name, "Stmt") {
				statements++
				statementType := attribute(value.Attr, "StatementType")
				if !strings.EqualFold(statementType, "SELECT") {
					return fmt.Errorf("SQL Server compiled a non-SELECT statement of type %q", statementType)
				}
			}
			if name == "RelOp" {
				logical := strings.ToLower(attribute(value.Attr, "LogicalOp"))
				physical := strings.ToLower(attribute(value.Attr, "PhysicalOp"))
				if planOperationHasSideEffect(logical) || planOperationHasSideEffect(physical) {
					return errors.New("SQL Server SHOWPLAN contains a data-modification operator")
				}
				if strings.Contains(logical, "remote") || strings.Contains(physical, "remote") || strings.Contains(logical, "external") || strings.Contains(physical, "external") {
					return errors.New("SQL Server SHOWPLAN contains a remote or external data operator")
				}
			}
			if name == "Object" {
				objectDatabase := unquotePlanIdentifier(attribute(value.Attr, "Database"))
				schema := unquotePlanIdentifier(attribute(value.Attr, "Schema"))
				table := unquotePlanIdentifier(attribute(value.Attr, "Table"))
				if objectDatabase != "" && !strings.EqualFold(objectDatabase, database) {
					return fmt.Errorf("SQL Server SHOWPLAN references database %q outside the selected target", objectDatabase)
				}
				if table != "" {
					if _, blocked := denied[strings.ToLower(table)]; blocked {
						return fmt.Errorf("SQL Server SHOWPLAN references denied table %q", table)
					}
					if _, blocked := denied[strings.ToLower(schema+"."+table)]; blocked {
						return fmt.Errorf("SQL Server SHOWPLAN references denied table %q", schema+"."+table)
					}
				}
			}
			stack = append(stack, name)
		case xml.EndElement:
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		}
	}
	if statements != 1 {
		return fmt.Errorf("SQL Server must compile exactly one SELECT statement; SHOWPLAN contained %d", statements)
	}
	return nil
}

func attribute(attributes []xml.Attr, name string) string {
	for _, item := range attributes {
		if item.Name.Local == name {
			return item.Value
		}
	}
	return ""
}

func unquotePlanIdentifier(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 2 && value[0] == '[' && value[len(value)-1] == ']' {
		return strings.ReplaceAll(value[1:len(value)-1], "]]", "]")
	}
	return value
}

func planOperationHasSideEffect(operation string) bool {
	for _, word := range []string{"insert", "update", "delete", "merge"} {
		if strings.Contains(operation, word) {
			return true
		}
	}
	return false
}

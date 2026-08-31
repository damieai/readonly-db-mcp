package mysql

import (
	"fmt"
	"regexp"
	"strings"
)

var grantLine = regexp.MustCompile(`(?i)^GRANT\s+(.+?)\s+ON\s+(.+?)\s+TO\s+.+$`)
var columnSelect = regexp.MustCompile(`(?i)^SELECT\s*\([^)]+\)$`)

func ValidateGrants(grants, allowedSchemas []string) error {
	allowed := lowerSet(allowedSchemas)
	for _, grant := range grants {
		upper := strings.ToUpper(strings.TrimSpace(grant))
		if strings.Contains(upper, " WITH GRANT OPTION") {
			return fmt.Errorf("account has GRANT OPTION")
		}
		if strings.HasPrefix(upper, "GRANT USAGE ON *.* TO ") {
			continue
		}
		parts := grantLine.FindStringSubmatch(strings.TrimSpace(grant))
		if len(parts) != 3 {
			return fmt.Errorf("unsupported grant shape; roles and proxy grants are refused")
		}
		privileges := strings.TrimSpace(parts[1])
		if !strings.EqualFold(privileges, "SELECT") && !columnSelect.MatchString(privileges) {
			return fmt.Errorf("account has privileges other than SELECT")
		}
		schema, ok := schemaFromGrantObject(parts[2])
		if !ok {
			return fmt.Errorf("cannot verify SELECT grant scope")
		}
		if _, permitted := allowed[strings.ToLower(schema)]; !permitted {
			return fmt.Errorf("account can SELECT outside configured schemas")
		}
	}
	return nil
}

func schemaFromGrantObject(object string) (string, bool) {
	object = strings.TrimSpace(object)
	parts := strings.SplitN(object, ".", 2)
	if len(parts) != 2 {
		return "", false
	}
	schema := unquoteIdentifier(parts[0])
	item := unquoteIdentifier(parts[1])
	if schema == "" || item == "" || schema == "*" {
		return "", false
	}
	return schema, true
}

func unquoteIdentifier(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 2 && value[0] == '`' && value[len(value)-1] == '`' {
		value = strings.ReplaceAll(value[1:len(value)-1], "``", "`")
	}
	return value
}

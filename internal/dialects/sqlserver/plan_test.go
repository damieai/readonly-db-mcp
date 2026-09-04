package sqlserver

import (
	"strings"
	"testing"
)

func selectPlan(database, schema, table string) string {
	return `<ShowPlanXML xmlns="http://schemas.microsoft.com/sqlserver/2004/07/showplan"><BatchSequence><Batch><Statements><StmtSimple StatementType="SELECT"><QueryPlan><RelOp LogicalOp="Index Scan" PhysicalOp="Index Scan"><IndexScan><Object Database="[` + database + `]" Schema="[` + schema + `]" Table="[` + table + `]" /></IndexScan></RelOp></QueryPlan></StmtSimple></Statements></Batch></BatchSequence></ShowPlanXML>`
}

func TestValidateShowPlanAcceptsOneSelect(t *testing.T) {
	if err := validateShowPlan(selectPlan("analytics", "reporting", "items"), "analytics", map[string]struct{}{}); err != nil {
		t.Fatal(err)
	}
}

func TestValidateShowPlanRejectsDataModificationOperator(t *testing.T) {
	plan := strings.Replace(selectPlan("analytics", "reporting", "items"), `LogicalOp="Index Scan"`, `LogicalOp="Update"`, 1)
	if err := validateShowPlan(plan, "analytics", map[string]struct{}{}); err == nil || !strings.Contains(err.Error(), "data-modification") {
		t.Fatalf("expected modification rejection, got %v", err)
	}
}

func TestValidateShowPlanRejectsCrossDatabaseObject(t *testing.T) {
	err := validateShowPlan(selectPlan("other", "reporting", "items"), "analytics", map[string]struct{}{})
	if err == nil || !strings.Contains(err.Error(), "outside") {
		t.Fatalf("expected cross-database rejection, got %v", err)
	}
}

func TestValidateShowPlanRejectsDeniedExpandedObject(t *testing.T) {
	err := validateShowPlan(selectPlan("analytics", "private", "secrets"), "analytics", map[string]struct{}{"private.secrets": {}})
	if err == nil || !strings.Contains(err.Error(), "denied") {
		t.Fatalf("expected denied-object rejection, got %v", err)
	}
}

func TestValidateShowPlanRequiresExactlyOneStatement(t *testing.T) {
	plan := strings.Replace(selectPlan("analytics", "reporting", "items"), "</Statements>", `<StmtSimple StatementType="SELECT"><QueryPlan /></StmtSimple></Statements>`, 1)
	if err := validateShowPlan(plan, "analytics", map[string]struct{}{}); err == nil || !strings.Contains(err.Error(), "exactly one") {
		t.Fatalf("expected single-statement rejection, got %v", err)
	}
}

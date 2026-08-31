package mysql

import (
	"strings"
	"testing"
)

func TestPolicyAllowsAdvancedReadOnlyQueries(t *testing.T) {
	policy := NewPolicy("inventory", []string{"inventory"}, nil, 32<<10)
	query := `
WITH movement_summary AS (
  SELECT company_id, product_id, SUM(quantity_in - quantity_out) AS balance
  FROM inventory.stock_movements
  WHERE company_id = ?
  GROUP BY company_id, product_id
)
SELECT *, RANK() OVER (ORDER BY balance DESC) AS stock_rank
FROM movement_summary
WHERE balance <> 0
ORDER BY stock_rank
LIMIT 500`
	validation, err := policy.Validate(query)
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if len(validation.Tables) != 1 || validation.Tables[0] != "inventory.stock_movements" {
		t.Fatalf("tables = %#v", validation.Tables)
	}
}

func TestPolicyRejectsMutationsAndMultipleStatements(t *testing.T) {
	policy := NewPolicy("inventory", []string{"inventory"}, nil, 32<<10)
	tests := []string{
		"UPDATE stock_movements SET quantity_in = 0",
		"DELETE FROM stock_movements",
		"DROP TABLE stock_movements",
		"SELECT 1; DELETE FROM stock_movements",
	}
	for _, query := range tests {
		t.Run(query, func(t *testing.T) {
			if _, err := policy.Validate(query); err == nil {
				t.Fatal("expected rejection")
			}
		})
	}
}

func TestPolicyRejectsSideEffects(t *testing.T) {
	policy := NewPolicy("inventory", []string{"inventory"}, nil, 32<<10)
	tests := []string{
		"SELECT * FROM stock_movements FOR UPDATE",
		"SELECT id INTO OUTFILE '/tmp/export' FROM stock_movements",
		"SELECT @value := id FROM stock_movements",
		"SELECT GET_LOCK('x', 10)",
		"SELECT SLEEP(10)",
		"SELECT LOAD_FILE('/etc/passwd')",
		"SELECT custom_side_effect(id) FROM stock_movements",
		"SELECT /*+ SET_VAR(max_execution_time=600000) */ id FROM stock_movements",
		"SELECT /*!50000 SLEEP(10) */ 1",
	}
	for _, query := range tests {
		t.Run(query, func(t *testing.T) {
			if _, err := policy.Validate(query); err == nil {
				t.Fatal("expected rejection")
			}
		})
	}
}

func TestPolicyRejectsOtherSchemas(t *testing.T) {
	policy := NewPolicy("inventory", []string{"inventory"}, nil, 32<<10)
	for _, query := range []string{
		"SELECT * FROM billing.invoices",
		"SELECT * FROM mysql.user",
		"SELECT * FROM performance_schema.threads",
	} {
		if _, err := policy.Validate(query); err == nil {
			t.Fatalf("expected %q to be rejected", query)
		}
	}
}

func TestPolicyAppliesQualifiedDenyToUnqualifiedTable(t *testing.T) {
	policy := NewPolicy("inventory", []string{"inventory"}, []string{"inventory.secrets"}, 32<<10)
	if _, err := policy.Validate("SELECT * FROM secrets"); err == nil {
		t.Fatal("expected the qualified deny rule to block an unqualified table")
	}
}

func TestPolicyDoesNotEchoRejectedSQL(t *testing.T) {
	policy := NewPolicy("inventory", []string{"inventory"}, nil, 32<<10)
	secret := "top-secret-literal"
	_, err := policy.Validate("SELECT unknown_function('" + secret + "')")
	if err == nil {
		t.Fatal("expected rejection")
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("error leaked SQL literal: %v", err)
	}
}

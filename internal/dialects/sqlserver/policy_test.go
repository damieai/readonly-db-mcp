package sqlserver

import (
	"strings"
	"testing"
)

func testPolicy() *Policy {
	return NewPolicy("Finance", "reporting", []string{"reporting", "dimensions"}, []string{"reporting.raw_payroll"}, 32<<10)
}

func TestPolicyAcceptsAdvancedReadOnlyTSQL(t *testing.T) {
	query := `
WITH regional AS (
  SELECT s.region, s.booked_at, s.amount
  FROM reporting.sales AS s WITH (UPDLOCK, ROWLOCK)
  WHERE s.booked_at >= @p1
)
SELECT TOP (@p2) region,
       SUM(amount) OVER (PARTITION BY region ORDER BY booked_at ROWS BETWEEN 6 PRECEDING AND CURRENT ROW) AS rolling,
       JSON_MODIFY(N'{"seen":false}', '$.seen', 1) AS payload
FROM regional
ORDER BY rolling DESC
OPTION (MAXRECURSION 1000, RECOMPILE);`
	v, err := testPolicy().Validate(query, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(v.Tables) != 1 || v.Tables[0] != "reporting.sales" {
		t.Fatalf("tables = %#v", v.Tables)
	}
}

func TestPolicyAcceptsDerivedTablesPivotAndLocalTVF(t *testing.T) {
	query := `
SELECT region,[2025],[2026]
FROM (
  SELECT region,year_value,amount
  FROM reporting.sales
) AS source
PIVOT (SUM(amount) FOR year_value IN ([2025],[2026])) AS p
CROSS APPLY Finance.reporting.currency_rate(region,@p1) AS rate`
	validation, err := testPolicy().Validate(query, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(validation.Tables) != 1 || validation.Tables[0] != "reporting.sales" {
		t.Fatalf("tables = %#v", validation.Tables)
	}
}

func TestPolicyRejectsPersistentEffects(t *testing.T) {
	tests := []struct {
		name, query, contains string
	}{
		{"select into", `SELECT * INTO reporting.copy FROM reporting.sales`, "SELECT INTO"},
		{"sequence", `SELECT NEXT VALUE FOR reporting.invoice_number`, "sequence"},
		{"cte delete", `WITH x AS (SELECT 1 AS id) DELETE FROM reporting.sales WHERE id IN (SELECT id FROM x)`, "delete"},
		{"second statement", `SELECT * FROM reporting.sales; DROP TABLE reporting.sales`, "exactly one"},
		{"external rowset", `SELECT * FROM OPENROWSET(BULK 'secret', SINGLE_CLOB) AS x`, "external"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := testPolicy().Validate(tt.query, 0)
			if err == nil || !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tt.contains)) {
				t.Fatalf("expected %q rejection, got %v", tt.contains, err)
			}
		})
	}
}

func TestPolicyDistinguishesStringsCommentsAndQuotedIdentifiers(t *testing.T) {
	query := `SELECT [delete], 'DROP TABLE x' AS note
FROM reporting.[update] -- MERGE reporting.sales
WHERE note = N'NEXT VALUE FOR ignored'`
	v, err := testPolicy().Validate(query, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(v.Tables) != 1 || v.Tables[0] != "reporting.update" {
		t.Fatalf("tables = %#v", v.Tables)
	}
}

func TestPolicyEnforcesParametersAndScope(t *testing.T) {
	for _, query := range []string{
		`SELECT * FROM other.sales`,
		`SELECT * FROM OtherDB.reporting.sales`,
		`SELECT * FROM Finance.reporting.raw_payroll`,
		`SELECT * FROM remote.Finance.reporting.sales`,
	} {
		if _, err := testPolicy().Validate(query, 0); err == nil {
			t.Fatalf("expected scope rejection for %q", query)
		}
	}
	if _, err := testPolicy().Validate(`SELECT * FROM reporting.sales WHERE id=@p2`, 1); err == nil {
		t.Fatal("expected non-contiguous parameter rejection")
	}
	if _, err := testPolicy().Validate(`SELECT * FROM reporting.sales WHERE id=@customer`, 1); err == nil {
		t.Fatal("expected native parameter naming rejection")
	}
}

func TestPolicyAllowsSafeNondeterminismButDisablesCache(t *testing.T) {
	v, err := testPolicy().Validate(`SELECT GETDATE(), NEWID(), @@VERSION`, 0)
	if err != nil {
		t.Fatal(err)
	}
	if v.Cacheable {
		t.Fatal("volatile query must not be cacheable")
	}
}

func TestLexerRejectsUnterminatedInput(t *testing.T) {
	for _, query := range []string{`SELECT 'oops`, `SELECT /* oops`, `SELECT [oops`} {
		if _, err := testPolicy().Validate(query, 0); err == nil {
			t.Fatalf("expected lexical rejection for %q", query)
		}
	}
}
